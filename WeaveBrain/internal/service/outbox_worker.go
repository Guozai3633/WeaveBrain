package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
)

// EnrichmentPipeline performs the background AI steps for a Capture: organize
// (memory-card fields), embed (vector), relate (connections), and recap.
// Every method must tolerate at-least-once invocation. R5 ships a no-op
// default; R6 and R8 replace it with real AI organization and completion.
type EnrichmentPipeline interface {
	Organize(ctx context.Context, capture *entity.Capture, card *entity.MemoryCard) error
	Embed(ctx context.Context, capture *entity.Capture, card *entity.MemoryCard) error
	Relate(ctx context.Context, capture *entity.Capture, card *entity.MemoryCard) error
	Recap(ctx context.Context, capture *entity.Capture, card *entity.MemoryCard) error
}

// defaultEnrichmentPipeline is the R5 no-op seam. Real enrichment lands in
// later rounds; the worker wiring and the switch gating are the R5 scope.
type defaultEnrichmentPipeline struct{}

// DefaultEnrichmentPipeline returns the no-op R5 pipeline.
func DefaultEnrichmentPipeline() EnrichmentPipeline { return &defaultEnrichmentPipeline{} }

func (p *defaultEnrichmentPipeline) Organize(context.Context, *entity.Capture, *entity.MemoryCard) error {
	return nil
}

func (p *defaultEnrichmentPipeline) Embed(context.Context, *entity.Capture, *entity.MemoryCard) error {
	return nil
}

func (p *defaultEnrichmentPipeline) Relate(context.Context, *entity.Capture, *entity.MemoryCard) error {
	return nil
}

func (p *defaultEnrichmentPipeline) Recap(context.Context, *entity.Capture, *entity.MemoryCard) error {
	return nil
}

const (
	// DefaultOutboxClaimBatch is the default number of events claimed per poll.
	DefaultOutboxClaimBatch = 20
	// DefaultOutboxPollInterval is the default worker poll cadence.
	DefaultOutboxPollInterval = time.Second
	// DefaultOutboxClaimLease is the default staleness window after which a
	// processing row left by a crashed worker is reclaimed by another worker.
	DefaultOutboxClaimLease = 5 * time.Minute

	outboxMaxAttempts   = 8
	outboxRetryBase     = time.Second
	outboxRetryMaxDelay = 5 * time.Minute
)

// OutboxWorkerConfig tunes the outbox worker; zero values use defaults.
type OutboxWorkerConfig struct {
	ClaimBatch   int
	PollInterval time.Duration
	// ClaimLease is the staleness window for reclaiming orphaned processing
	// rows left by a crashed worker. Defaults to DefaultOutboxClaimLease.
	ClaimLease  time.Duration
	MaxAttempts int
	RetryBase   time.Duration
	RetryMax    time.Duration
}

func (c OutboxWorkerConfig) withDefaults() OutboxWorkerConfig {
	if c.ClaimBatch <= 0 {
		c.ClaimBatch = DefaultOutboxClaimBatch
	}
	if c.PollInterval <= 0 {
		c.PollInterval = DefaultOutboxPollInterval
	}
	if c.ClaimLease <= 0 {
		c.ClaimLease = DefaultOutboxClaimLease
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = outboxMaxAttempts
	}
	if c.RetryBase <= 0 {
		c.RetryBase = outboxRetryBase
	}
	if c.RetryMax <= 0 {
		c.RetryMax = outboxRetryMaxDelay
	}
	return c
}

// OutboxWorker polls capture_outbox and enriches each Capture according to the
// policy snapshot recorded at creation time. When AI memory organizing was
// disabled in that snapshot (or the Capture was marked no_ai), the worker
// simply marks the event ready without calling the pipeline.
type OutboxWorker struct {
	outbox   repository.OutboxRepository
	captures repository.CaptureRepository
	pipeline EnrichmentPipeline
	config   OutboxWorkerConfig
	logger   *log.Logger

	cancel context.CancelFunc
	done   chan struct{}
}

// NewOutboxWorker creates a worker. Pipeline may be nil (events are then
// marked ready without enrichment). Config zero values fall back to defaults.
func NewOutboxWorker(
	outbox repository.OutboxRepository,
	captures repository.CaptureRepository,
	pipeline EnrichmentPipeline,
	config OutboxWorkerConfig,
) *OutboxWorker {
	return &OutboxWorker{
		outbox:   outbox,
		captures: captures,
		pipeline: pipeline,
		config:   config.withDefaults(),
		logger:   log.Default(),
	}
}

// Start launches the worker loop in a background goroutine.
func (w *OutboxWorker) Start() {
	if w == nil || w.done != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go w.run(ctx)
}

// Stop gracefully stops the worker loop.
func (w *OutboxWorker) Stop() {
	if w == nil || w.cancel == nil {
		return
	}
	w.cancel()
	<-w.done
}

func (w *OutboxWorker) run(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.processDue(ctx); err != nil {
				w.logger.Printf("outbox worker: process due events: %v", err)
			}
		}
	}
}

// processDue claims one batch and processes each event.
func (w *OutboxWorker) processDue(ctx context.Context) error {
	events, err := w.outbox.ClaimDue(ctx, w.config.ClaimBatch, w.config.ClaimLease)
	if err != nil {
		return err
	}
	for _, event := range events {
		w.processOne(ctx, event)
	}
	return nil
}

func (w *OutboxWorker) processOne(ctx context.Context, event *entity.CaptureOutbox) {
	snapshot := event.PolicySnapshot
	if snapshot == nil {
		snapshot = entity.DefaultPolicySnapshot()
	}
	privacyMode, _ := event.Payload["privacy_mode"].(string)
	if privacyMode == "" {
		privacyMode = "cloud_allowed"
	}
	if !snapshot.AIMemoryEnabled || privacyMode == "no_ai" {
		if err := w.outbox.MarkReady(ctx, event.ID); err != nil {
			w.logger.Printf("outbox worker: mark event %d ready (skipped): %v", event.ID, err)
		}
		return
	}

	capture, err := w.captures.GetByID(ctx, event.UserID, event.CaptureID)
	if err != nil {
		if errors.Is(err, repository.ErrCaptureNotFound) {
			// The capture no longer exists; retrying will never help.
			if markErr := w.outbox.MarkFailed(ctx, event.ID, "capture not found"); markErr != nil {
				w.logger.Printf("outbox worker: mark event %d failed: %v", event.ID, markErr)
			}
			return
		}
		w.scheduleRetry(ctx, event, fmt.Errorf("load capture: %w", err))
		return
	}

	if err := w.enrich(ctx, event, capture); err != nil {
		w.scheduleRetry(ctx, event, err)
		return
	}
	if err := w.outbox.MarkReady(ctx, event.ID); err != nil {
		w.logger.Printf("outbox worker: mark event %d ready: %v", event.ID, err)
	}
}

func (w *OutboxWorker) enrich(
	ctx context.Context,
	_ *entity.CaptureOutbox,
	capture *entity.CaptureAggregate,
) error {
	if w.pipeline == nil {
		return nil
	}
	if err := w.pipeline.Organize(ctx, capture.Capture, capture.MemoryCard); err != nil {
		return fmt.Errorf("organize: %w", err)
	}
	if err := w.pipeline.Embed(ctx, capture.Capture, capture.MemoryCard); err != nil {
		return fmt.Errorf("embed: %w", err)
	}
	if err := w.pipeline.Relate(ctx, capture.Capture, capture.MemoryCard); err != nil {
		return fmt.Errorf("relate: %w", err)
	}
	if err := w.pipeline.Recap(ctx, capture.Capture, capture.MemoryCard); err != nil {
		return fmt.Errorf("recap: %w", err)
	}
	return nil
}

func (w *OutboxWorker) scheduleRetry(ctx context.Context, event *entity.CaptureOutbox, err error) {
	if event.AttemptCount >= w.config.MaxAttempts {
		if markErr := w.outbox.MarkFailed(ctx, event.ID, err.Error()); markErr != nil {
			w.logger.Printf("outbox worker: mark event %d failed: %v", event.ID, markErr)
		}
		return
	}
	backoff := w.retryBackoff(event.AttemptCount)
	if markErr := w.outbox.MarkRetryWait(ctx, event.ID, backoff, err.Error()); markErr != nil {
		w.logger.Printf("outbox worker: mark event %d retry_wait: %v", event.ID, markErr)
	}
}

// retryBackoff returns the exponential backoff for a failed attempt.
func (w *OutboxWorker) retryBackoff(attempt int) time.Duration {
	shift := attempt - 1
	if shift < 0 {
		shift = 0
	}
	if shift > 20 {
		shift = 20
	}
	delay := w.config.RetryBase * time.Duration(1<<shift)
	if delay > w.config.RetryMax {
		return w.config.RetryMax
	}
	return delay
}
