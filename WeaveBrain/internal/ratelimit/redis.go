package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisLimiter implements rate limiting using Redis sorted sets (sliding window).
type RedisLimiter struct {
	client *redis.Client
	cfg    Config
}

// NewRedisLimiter creates a new Redis-backed rate limiter.
func NewRedisLimiter(client *redis.Client, cfg Config) *RedisLimiter {
	return &RedisLimiter{client: client, cfg: cfg}
}

// Allow checks whether a request for the given key is within the rate limit.
// Uses ZREMRANGEBYSCORE + ZCARD + ZADD for atomic sliding window counting.
func (l *RedisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-l.cfg.Window)
	rateLimitKey := fmt.Sprintf("ratelimit:%s", key)

	pipe := l.client.Pipeline()

	// Remove expired entries outside the window
	pipe.ZRemRangeByScore(ctx, rateLimitKey, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Count current entries in the window
	cardCmd := pipe.ZCard(ctx, rateLimitKey)

	// Add the current request timestamp
	pipe.ZAdd(ctx, rateLimitKey, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})

	// Set expiry on the key to auto-cleanup
	pipe.Expire(ctx, rateLimitKey, l.cfg.Window+time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("ratelimit: redis pipeline failed: %w", err)
	}

	count := cardCmd.Val()
	return count < int64(l.cfg.MaxRequests), nil
}

// Close is a no-op; the Redis client lifecycle is managed externally.
func (l *RedisLimiter) Close() error {
	return nil
}
