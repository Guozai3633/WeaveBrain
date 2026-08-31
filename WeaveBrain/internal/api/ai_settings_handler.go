package api

import (
	"context"
	"errors"
	"net/http"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxAISettingsBodyBytes = 1 << 16 // 64 KiB

type aiSettingsUseCase interface {
	Get(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error)
	Update(ctx context.Context, input service.UpdateAISettingsInput) (*entity.UserAISettings, error)
	CountPendingReorganize(ctx context.Context, userID uuid.UUID) (int64, error)
	Reorganize(ctx context.Context, userID uuid.UUID) (int64, error)
}

// AISettingsHandler exposes the per-user AI consent switches.
type AISettingsHandler struct {
	service aiSettingsUseCase
}

// UpdateAISettingsRequest is a partial PATCH. Pointer fields are applied only
// when present; expected_revision must be supplied for optimistic concurrency.
type UpdateAISettingsRequest struct {
	ExpectedRevision    *int64 `json:"expected_revision"`
	AIMemoryEnabled     *bool  `json:"ai_memory_enabled"`
	AICompletionEnabled *bool  `json:"ai_completion_enabled"`
	SpeechToTextEnabled *bool  `json:"speech_to_text_enabled"`
	CloudTextAllowed    *bool  `json:"cloud_text_allowed"`
	CloudAudioAllowed   *bool  `json:"cloud_audio_allowed"`
}

// AISettingsResponse is the shared envelope for GET and PATCH.
type AISettingsResponse struct {
	Settings          *entity.UserAISettings `json:"settings"`
	PendingReorganize int64                  `json:"pending_reorganize"`
	RequestID         string                 `json:"request_id"`
}

// ReorganizeResponse reports how many past memories were explicitly re-enqueued.
type ReorganizeResponse struct {
	Reorganized int64  `json:"reorganized"`
	RequestID   string `json:"request_id"`
}

func NewAISettingsHandler(aiSettingsService aiSettingsUseCase) *AISettingsHandler {
	return &AISettingsHandler{service: aiSettingsService}
}

func (h *AISettingsHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/users/me/ai-settings", h.Get)
	group.PATCH("/users/me/ai-settings", h.Update)
	group.POST("/users/me/ai-settings/reorganize", h.Reorganize)
}

func (h *AISettingsHandler) Get(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	settings, err := h.service.Get(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	pending, err := h.service.CountPendingReorganize(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, AISettingsResponse{
		Settings:          settings,
		PendingReorganize: pending,
		RequestID:         getRequestID(c),
	})
}

// Reorganize explicitly re-enqueues the user's memories created while AI
// organizing was off. It requires AI memory organizing to be currently enabled.
func (h *AISettingsHandler) Reorganize(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	reorganized, err := h.service.Reorganize(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ReorganizeResponse{Reorganized: reorganized, RequestID: getRequestID(c)})
}

func (h *AISettingsHandler) Update(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAISettingsBodyBytes)
	var request UpdateAISettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"invalid request body",
			map[string]any{"field": "body"},
		)
		return
	}
	if request.ExpectedRevision == nil || *request.ExpectedRevision < 0 {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"expected_revision is required and must be non-negative",
			map[string]any{"field": "expected_revision"},
		)
		return
	}
	if !hasAnyAISettingsField(request) {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"at least one settings field is required",
			map[string]any{"field": "body"},
		)
		return
	}

	updated, err := h.service.Update(c.Request.Context(), service.UpdateAISettingsInput{
		UserID:              userID,
		ExpectedRevision:    *request.ExpectedRevision,
		AIMemoryEnabled:     request.AIMemoryEnabled,
		AICompletionEnabled: request.AICompletionEnabled,
		SpeechToTextEnabled: request.SpeechToTextEnabled,
		CloudTextAllowed:    request.CloudTextAllowed,
		CloudAudioAllowed:   request.CloudAudioAllowed,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, AISettingsResponse{Settings: updated, RequestID: getRequestID(c)})
}

func (h *AISettingsHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidAISettings):
		writeV3Error(c, http.StatusBadRequest, V3ErrorInvalidArgument, err.Error(), nil)
	case errors.Is(err, service.ErrAISettingsConflict):
		writeV3Error(c, http.StatusConflict, V3ErrorVersionConflict, err.Error(), nil)
	case errors.Is(err, service.ErrReorganizeAIMemoryDisabled):
		writeV3Error(c, http.StatusConflict, V3ErrorFeatureNotEnabled, err.Error(), nil)
	default:
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "internal server error", nil)
	}
}

func hasAnyAISettingsField(request UpdateAISettingsRequest) bool {
	return request.AIMemoryEnabled != nil ||
		request.AICompletionEnabled != nil ||
		request.SpeechToTextEnabled != nil ||
		request.CloudTextAllowed != nil ||
		request.CloudAudioAllowed != nil
}
