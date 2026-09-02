package api

import (
	"context"
	"net/http"

	"weavebrain/internal/ratelimit"
	"weavebrain/internal/review"
	"weavebrain/internal/service"
	"weavebrain/pkg/auth"

	"github.com/gin-gonic/gin"
)

// Server holds the HTTP server configuration and routes.
type Server struct {
	port          string
	services      *service.Services
	tokenCfg      auth.TokenConfig
	engine        *gin.Engine
	http          *http.Server
	sttProvider   string
	reviewService *review.Service
	rateLimiter   ratelimit.Limiter
	encryptionKey []byte
}

// NewServer creates a new HTTP server with routes.
func NewServer(port string, services *service.Services, tokenCfg auth.TokenConfig, sttProvider string, reviewService *review.Service, rateLimiter ratelimit.Limiter, encryptionKey []byte) *Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()
	engine.Use(requestIDMiddleware())

	// CORS middleware
	engine.Use(corsMiddleware())

	// Recovery middleware
	engine.Use(gin.Recovery())

	// Rate limiting middleware
	if rateLimiter != nil {
		engine.Use(ratelimit.Middleware(rateLimiter))
	}

	s := &Server{
		port:          port,
		services:      services,
		tokenCfg:      tokenCfg,
		engine:        engine,
		sttProvider:   sttProvider,
		reviewService: reviewService,
		rateLimiter:   rateLimiter,
		encryptionKey: encryptionKey,
	}

	s.setupRoutes()

	s.http = &http.Server{
		Addr:    ":" + port,
		Handler: s.engine,
	}

	return s
}

func (s *Server) setupRoutes() {
	registerV3ContractRoutes(s.engine)

	// New domain routes use the V3 error and authentication contracts.
	protectedV3 := s.engine.Group(apiV3Prefix)
	protectedV3.Use(v3JWTMiddleware(s.tokenCfg))
	{
		captureHandler := NewCaptureHandler(s.services.Capture)
		captureHandler.RegisterRoutes(protectedV3)

		if s.services.Audio != nil {
			audioHandler := NewAudioAssetHandler(s.services.Audio)
			audioHandler.RegisterRoutes(protectedV3)
		}

		if s.services.AISettings != nil {
			aiSettingsHandler := NewAISettingsHandler(s.services.AISettings)
			aiSettingsHandler.RegisterRoutes(protectedV3)
		}

		if s.services.Memory != nil {
			memoryHandler := NewMemoryHandler(s.services.Memory)
			memoryHandler.RegisterRoutes(protectedV3)
		}

		if s.services.Completion != nil {
			completionHandler := NewCompletionHandler(s.services.Completion)
			completionHandler.RegisterRoutes(protectedV3)
		}

		if s.services.Import != nil {
			importHandler := NewImportHandler(s.services.Import)
			importHandler.RegisterRoutes(protectedV3)
		}
	}

	// Public routes (no auth required)
	public := s.engine.Group("/api/v1")
	{
		authGroup := public.Group("/auth")
		s.setupAuthRoutes(authGroup)
	}

	// Protected routes (JWT required)
	protected := s.engine.Group("/api/v1")
	protected.Use(auth.JWTMiddleware(s.tokenCfg))
	{
		users := protected.Group("/users")
		s.setupUserRoutes(users)

		// 注册 users/me/mcp-configs
		usersMe := users.Group("/me")
		mcpHandler := NewMcpConfigHandler(s.services.Store, s.encryptionKey)
		mcpHandler.RegisterRoutes(usersMe)

		// 注册 users/me/timeline
		timelineHandler := NewTimelineHandler(s.services.Store)
		usersMe.GET("/timeline", timelineHandler.GetTimeline)

		projects := protected.Group("/projects")
		s.setupProjectRoutes(projects)

		ideas := protected.Group("/ideas")
		s.setupIdeaRoutes(ideas)

		reminders := protected.Group("/reminders")
		s.setupReminderRoutes(reminders)

		// Agent
		agentHandler := NewAgentHandler(s.services.Agent)
		agentHandler.RegisterRoutes(protected)

		// Config
		config := protected.Group("/config")
		s.setupConfigRoutes(config)
	}

	// Voice WebSocket (JWT via query param)
	if s.services.STT != nil {
		voiceHandler := NewVoiceWSHandler(s.services.STT, s.reviewService, s.tokenCfg)
		voiceHandler.RegisterRoute(s.engine)
	}

	// Health check (public)
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func (s *Server) setupConfigRoutes(rg *gin.RouterGroup) {
	rg.GET("/stt", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"provider": s.sttProvider})
	})
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	return s.http.ListenAndServe()
}

// ShutdownWithContext gracefully shuts down the server.
func (s *Server) ShutdownWithContext(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}
