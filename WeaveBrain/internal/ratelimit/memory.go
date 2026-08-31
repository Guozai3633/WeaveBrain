package ratelimit

import (
	"context"
	"sync"
	"time"
)

// MemoryLimiter implements rate limiting using an in-memory token bucket algorithm.
// Used as a fallback when Redis is unavailable.
type MemoryLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	cfg      Config
	stopOnce sync.Once
	stopCh   chan struct{}
}

type bucket struct {
	tokens     float64
	lastFill   time.Time
	maxTokens  float64
	refillRate float64 // tokens per second
}

// NewMemoryLimiter creates a new in-memory rate limiter.
func NewMemoryLimiter(cfg Config) *MemoryLimiter {
	l := &MemoryLimiter{
		buckets: make(map[string]*bucket),
		cfg:     cfg,
		stopCh:  make(chan struct{}),
	}
	go l.cleanup()
	return l
}

// Allow checks whether a request for the given key is within the rate limit.
func (l *MemoryLimiter) Allow(_ context.Context, key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists := l.buckets[key]
	if !exists {
		b = &bucket{
			tokens:     float64(l.cfg.MaxRequests),
			lastFill:   time.Now(),
			maxTokens:  float64(l.cfg.MaxRequests),
			refillRate: float64(l.cfg.MaxRequests) / l.cfg.Window.Seconds(),
		}
		l.buckets[key] = b
	}

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastFill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, nil
	}

	return false, nil
}

// Close stops the background cleanup goroutine.
func (l *MemoryLimiter) Close() error {
	l.stopOnce.Do(func() {
		close(l.stopCh)
	})
	return nil
}

// cleanup periodically removes stale buckets to prevent memory leaks.
func (l *MemoryLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-l.stopCh:
			return
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for key, b := range l.buckets {
				if now.Sub(b.lastFill) > l.cfg.Window*2 {
					delete(l.buckets, key)
				}
			}
			l.mu.Unlock()
		}
	}
}
