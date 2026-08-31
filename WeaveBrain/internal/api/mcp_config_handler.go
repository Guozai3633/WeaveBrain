package api

import (
	"net/http"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/pkg/crypto"

	"github.com/gin-gonic/gin"
)

type McpConfigHandler struct {
	store         *repository.DBStore
	encryptionKey []byte
}

func NewMcpConfigHandler(store *repository.DBStore, encryptionKey []byte) *McpConfigHandler {
	return &McpConfigHandler{
		store:         store,
		encryptionKey: encryptionKey,
	}
}

func (h *McpConfigHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/mcp-configs", h.handleListConfigs)
	rg.PUT("/mcp-configs/:namespace", h.handleUpsertConfig)
	rg.DELETE("/mcp-configs/:namespace", h.handleDeleteConfig)
}

type McpConfigResponse struct {
	Namespace    string `json:"namespace"`
	Enabled      bool   `json:"enabled"`
	IsConfigured bool   `json:"is_configured"` // true if there's any credential saved
}

func (h *McpConfigHandler) handleListConfigs(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	configs, err := h.store.UserMcpConfig.ListByUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve configs"})
		return
	}

	var response []McpConfigResponse
	for _, conf := range configs {
		response = append(response, McpConfigResponse{
			Namespace:    conf.ToolNamespace,
			Enabled:      conf.Enabled,
			IsConfigured: len(conf.EncryptedCredentials) > 0,
		})
	}

	c.JSON(http.StatusOK, response)
}

type UpsertMcpConfigRequest struct {
	Credentials string `json:"credentials" binding:"required"`
	Enabled     bool   `json:"enabled"`
}

func (h *McpConfigHandler) handleUpsertConfig(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	namespace := c.Param("namespace")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace is required"})
		return
	}

	var req UpsertMcpConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Encrypt credentials
	encrypted, err := crypto.Encrypt([]byte(req.Credentials), h.encryptionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encrypt credentials"})
		return
	}

	config := &entity.UserMcpConfig{
		UserID:               userID,
		ToolNamespace:        namespace,
		EncryptedCredentials: encrypted,
		Enabled:              req.Enabled,
	}

	if err := h.store.UserMcpConfig.Upsert(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *McpConfigHandler) handleDeleteConfig(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	namespace := c.Param("namespace")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namespace is required"})
		return
	}

	if err := h.store.UserMcpConfig.Delete(c.Request.Context(), userID, namespace); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
