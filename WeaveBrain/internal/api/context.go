package api

import (
	"context"

	"weavebrain/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SetCurrentUserID sets the current user ID in the request context.
func SetCurrentUserID(c *gin.Context, userID uuid.UUID) {
	c.Set(auth.ContextKeyUserID, userID)
}

// GetCurrentUserID retrieves the current user ID from the request context.
func getCurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	val, ok := c.Get(auth.ContextKeyUserID)
	if !ok {
		return uuid.Nil, false
	}
	userID, ok := val.(uuid.UUID)
	return userID, ok
}

// GetCurrentUserIDFromCtx retrieves the user ID from a raw context.
func GetCurrentUserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	val := ctx.Value(auth.ContextKeyUserID)
	if val == nil {
		return uuid.Nil, false
	}
	userID, ok := val.(uuid.UUID)
	return userID, ok
}
