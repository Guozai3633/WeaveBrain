package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"weavebrain/internal/api"
	"weavebrain/internal/db/repository"
	"weavebrain/internal/embedding"
	ollamaemb "weavebrain/internal/embedding/ollama"
	"weavebrain/internal/ratelimit"
	"weavebrain/internal/review"
	mockreview "weavebrain/internal/review/mock"
	llmreview "weavebrain/internal/review/llm"
	"weavebrain/internal/service"
	"weavebrain/internal/stt"
	funasrstt "weavebrain/internal/stt/funasr"
	mockstt "weavebrain/internal/stt/mock"
	"weavebrain/internal/workflow"
	"weavebrain/pkg/auth"
	"weavebrain/internal/agent"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Handle subcommands
	if len(os.Args) > 1 && os.Args[1] == "backfill-embeddings" {
		runBackfillEmbeddings()
		return
	}

	// Load config from environment
	port := getEnvOrDefault("SERVER_PORT", "8080")

	log.Printf("Starting weavebrain on :%s", port)

	// Initialize database connection pool
	pool, err := pgxpool.New(context.Background(), getDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Println("Database connected successfully")

	// Initialize repositories via DBStore
	repos := repository.NewFromPool(pool)

	// Initialize services
	services := service.New(repos)

	// Initialize STT provider (configurable via STT_PROVIDER env)
	var sttProvider stt.STTProvider
	sttProviderName := getEnvOrDefault("STT_PROVIDER", "mock")
	switch sttProviderName {
	case "funasr":
		funasrAddr := getEnvOrDefault("FUNASR_ADDR", "localhost:10095")
		sttProvider = funasrstt.NewProvider(funasrstt.Config{
			ServerAddr: funasrAddr,
			SampleRate: 16000,
		})
		log.Printf("STT service initialized (FunASR provider, addr=%s)", funasrAddr)
	default:
		sttProvider = mockstt.NewProvider(mockstt.Config{})
		log.Println("STT service initialized (mock provider)")
	}
	services.STT = service.NewSTTService(sttProvider)

	// Initialize Review provider (configurable via REVIEW_PROVIDER env)
	var reviewSvc *review.Service
	reviewProviderName := getEnvOrDefault("REVIEW_PROVIDER", "mock")
	switch reviewProviderName {
	case "llm":
		llmCfg := agent.DefaultLLMConfig()
		llmReviewProv, err := llmreview.NewProvider(context.Background(), llmCfg)
		if err != nil {
			log.Printf("Warning: LLM review provider failed to init: %v", err)
			log.Println("Falling back to mock review provider")
			reviewSvc = review.NewService(mockreview.NewProvider())
		} else {
			reviewSvc = review.NewService(llmReviewProv)
			log.Printf("Review service initialized (LLM provider, model=%s)", llmCfg.Model)
		}
	default:
		reviewSvc = review.NewService(mockreview.NewProvider())
		log.Println("Review service initialized (mock provider)")
	}

	// Initialize Rate Limiter (Redis with memory fallback)
	rateLimitCfg := ratelimit.DefaultConfig()
	var limiter ratelimit.Limiter
	redisURL := getEnvOrDefault("REDIS_URL", "redis://localhost:6379")
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("Warning: Invalid REDIS_URL, using memory rate limiter: %v", err)
		limiter = ratelimit.NewMemoryLimiter(rateLimitCfg)
	} else {
		redisClient := redis.NewClient(opt)
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Printf("Warning: Redis unavailable, using memory rate limiter: %v", err)
			limiter = ratelimit.NewMemoryLimiter(rateLimitCfg)
		} else {
			limiter = ratelimit.NewRedisLimiter(redisClient, rateLimitCfg)
			log.Printf("Rate limiter initialized (Redis, %d req/%v)", rateLimitCfg.MaxRequests, rateLimitCfg.Window)
		}
	}

	// Initialize Embedding provider (configurable via EMBEDDING_BASE_URL env)
	var embProvider embedding.Provider
	embDim, _ := strconv.Atoi(getEnvOrDefault("EMBEDDING_DIM", "768"))
	embModel := getEnvOrDefault("EMBEDDING_MODEL", "nomic-embed-text")
	embBaseURL := getEnvOrDefault("EMBEDDING_BASE_URL", "http://localhost:11434/v1")
	embAPIKey := getEnvOrDefault("EMBEDDING_API_KEY", "ollama")

	embProvider = ollamaemb.NewProvider(ollamaemb.Config{
		BaseURL: embBaseURL,
		APIKey:  embAPIKey,
		Model:   embModel,
		Dim:     embDim,
	})
	defer embProvider.Close()
	log.Printf("Embedding service initialized (Ollama provider, model=%s, dim=%d)", embModel, embDim)

	embeddingSvc := service.NewEmbeddingService(embProvider, repos)
	services.Embedding = embeddingSvc
	services.Idea.SetEmbeddingService(embeddingSvc)
	services.Agent.SetEmbeddingService(embeddingSvc)

	// Initialize Agent (non-blocking, can fail gracefully if LLM not available)
	go func() {
		if err := services.Agent.Init(context.Background()); err != nil {
			log.Printf("Warning: Agent initialization failed: %v", err)
			log.Println("Agent endpoints will return 503 until initialized")
		} else {
			log.Println("Agent initialized successfully")
		}
	}()

	// Initialize Temporal (graceful degradation if unavailable)
	var temporalWorker *workflow.Worker
	temporalClient, err := workflow.NewTemporalClient("")
	if err != nil {
		log.Printf("Warning: Temporal connection failed: %v", err)
		log.Println("Running in synchronous mode (no workflow orchestration)")
	} else {
		log.Println("Temporal connected successfully")

		activities := &workflow.Activities{
			AgentService:    services.Agent,
			IdeaService:     services.Idea,
			ReminderService: services.Reminder,
			ProjectService:  services.Project,
		}

		temporalWorker = workflow.StartWorker(temporalClient, activities)

		dispatcher := workflow.NewDispatcher(temporalClient, repos)
		services.Agent.SetDispatcher(dispatcher)

		// Set up cron schedules for recurring workflows
		scheduleMgr := workflow.NewScheduleManager(temporalClient)
		if err := scheduleMgr.SetupSchedules(context.Background()); err != nil {
			log.Printf("Warning: Failed to setup schedules: %v", err)
		}

		log.Println("Temporal workflows registered")
	}

	// JWT configuration
	tokenCfg := auth.TokenConfig{
		Secret: []byte(getEnvOrDefault("JWT_SECRET", "dev-secret-change-in-production")),
		Issuer: "weavebrain",
		Expiry: 72 * time.Hour,
	}

	// Initialize HTTP server
	server := api.NewServer(port, services, tokenCfg, sttProviderName, reviewSvc, limiter)

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil {
			if err == http.ErrServerClosed {
				log.Println("Server shut down gracefully")
				return
			}
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down...")

	if temporalWorker != nil {
		temporalWorker.Stop()
		log.Println("Temporal worker stopped")
	}
	if temporalClient != nil {
		temporalClient.Close()
	}
	if services.STT != nil {
		services.STT.Close()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.ShutdownWithContext(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped")
}

func getDSN() string {
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "5432")
	user := getEnvOrDefault("DB_USER", "weavebrain")
	password := getEnvOrDefault("DB_PASSWORD", "weavebrain_dev")
	dbname := getEnvOrDefault("DB_NAME", "weavebrain")

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func runBackfillEmbeddings() {
	log.Println("Starting embedding backfill...")

	pool, err := pgxpool.New(context.Background(), getDSN())
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	repos := repository.NewFromPool(pool)

	embDim, _ := strconv.Atoi(getEnvOrDefault("EMBEDDING_DIM", "768"))
	embModel := getEnvOrDefault("EMBEDDING_MODEL", "nomic-embed-text")
	embBaseURL := getEnvOrDefault("EMBEDDING_BASE_URL", "http://localhost:11434/v1")
	embAPIKey := getEnvOrDefault("EMBEDDING_API_KEY", "ollama")

	provider := ollamaemb.NewProvider(ollamaemb.Config{
		BaseURL: embBaseURL,
		APIKey:  embAPIKey,
		Model:   embModel,
		Dim:     embDim,
	})
	defer provider.Close()

	embeddingSvc := service.NewEmbeddingService(provider, repos)

	total := 0
	for {
		processed, err := embeddingSvc.BackfillEmbeddings(context.Background(), 50)
		if err != nil {
			log.Fatalf("Backfill failed: %v", err)
		}
		total += processed
		if processed == 0 {
			break
		}
		log.Printf("Processed %d ideas (total: %d)", processed, total)
	}

	log.Printf("Backfill complete. Total ideas processed: %d", total)
}
