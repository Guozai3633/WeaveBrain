package api

import (
	"net/http"

	"weavebrain/internal/entity"
	"weavebrain/pkg/auth"

	"github.com/gin-gonic/gin"
)

func (s *Server) setupAuthRoutes(group *gin.RouterGroup) {
	group.POST("/register", s.handleRegister)
	group.POST("/login", s.handleLogin)
}

// RegisterRequest represents the request body for user registration.
type RegisterRequest struct {
	Provider    string  `json:"provider" binding:"required"`
	ProviderID  string  `json:"provider_id" binding:"required"`
	DisplayName *string `json:"display_name,omitempty"`
	Phone       *string `json:"phone,omitempty"`
}

// LoginRequest represents the request body for user login.
type LoginRequest struct {
	Provider   string `json:"provider" binding:"required"`
	ProviderID string `json:"provider_id" binding:"required"`
}

func (s *Server) handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	user, err := s.services.User.GetOrCreateByProvider(c.Request.Context(), req.Provider, req.ProviderID, req.Phone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	token, err := auth.GenerateToken(s.tokenCfg, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user":  toUserResponse(user),
		"token": token,
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	identity, err := s.services.Identity.GetByProvider(c.Request.Context(), req.Provider, req.ProviderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found, please register"})
		return
	}

	user, err := s.services.User.GetUserByID(c.Request.Context(), identity.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve user"})
		return
	}

	token, err := auth.GenerateToken(s.tokenCfg, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  toUserResponse(user),
		"token": token,
	})
}

func toUserResponse(u *entity.User) map[string]any {
	resp := map[string]any{
		"id":         u.ID.String(),
		"created_at": u.CreatedAt,
		"updated_at": u.UpdatedAt,
	}
	if u.DisplayName != nil {
		resp["display_name"] = *u.DisplayName
	}
	if u.AvatarURL != nil {
		resp["avatar_url"] = *u.AvatarURL
	}
	return resp
}
