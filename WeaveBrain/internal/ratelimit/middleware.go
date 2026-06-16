package ratelimit

import (
	"log"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Middleware returns a Gin middleware that applies rate limiting.
// Requests are keyed by client IP for anonymous requests, or by user ID for authenticated requests.
// If the limiter returns an error, the request is allowed through (fail-open).
func Middleware(limiter Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := clientKey(c)

		allowed, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			// Fail-open: log the error and allow the request
			log.Printf("[RateLimit] Error checking limit for %s: %v", key, err)
			c.Next()
			return
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "RATE_LIMITED",
					"message": "Too many requests. Please try again later.",
				},
			})
			return
		}

		c.Next()
	}
}

// clientKey extracts a rate limit key from the request context.
// Prefers authenticated user ID, falls back to client IP.
func clientKey(c *gin.Context) string {
	// Try to get user ID from JWT context (set by auth middleware)
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(string); ok && uid != "" {
			return "user:" + uid
		}
	}

	// Fall back to client IP
	ip := c.ClientIP()
	if ip == "" {
		ip = "unknown"
	}

	// Normalize to remove port if present
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}

	return "ip:" + ip
}
