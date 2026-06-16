package ratelimit

import (
	"context"
	"time"
)

// Limiter defines the interface for rate limiting backends.
type Limiter interface {
	// Allow checks whether a request for the given key is allowed.
	// Returns true if the request is within the rate limit, false otherwise.
	Allow(ctx context.Context, key string) (bool, error)

	// Close releases any resources held by the limiter.
	Close() error
}

// Config holds rate limiter configuration.
type Config struct {
	// MaxRequests is the maximum number of requests allowed in the window.
	MaxRequests int

	// Window is the sliding time window duration.
	Window time.Duration
}

// DefaultConfig returns a sensible default configuration (100 req/min).
func DefaultConfig() Config {
	return Config{
		MaxRequests: 100,
		Window:      time.Minute,
	}
}
