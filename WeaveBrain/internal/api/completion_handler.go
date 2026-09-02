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

// completionUseCase is the service surface the handler needs.
type completionUseCase interface {
	Preview(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.CompletionPreviewResult, error)
	Apply(ctx context.Context, userID uuid.UUID, captureID uuid.UUID, input service.ApplyCompletionInput) (*entity.CompletionApplyResult, error)
	Undo(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.CompletionApplyResult, error)
}

// CompletionHandler exposes the AI field-completion endpoints: preview
// (no mutation), apply (fill empty fields), undo (revert the last apply).
type CompletionHandler struct {
	service completionUseCase
}

type CompletionPreviewResponse struct {
	Preview   *entity.CompletionPreviewResult `json:"preview"`
	RequestID string                          `json:"request_id"`
}

type CompletionApplyResponse struct {
	Apply     *entity.CompletionApplyResult `json:"apply"`
	RequestID string                        `json:"request_id"`
}

type CompletionUndoResponse struct {
	Undo      *entity.CompletionApplyResult `json:"undo"`
	RequestID string                        `json:"request_id"`
}

func NewCompletionHandler(completionService completionUseCase) *CompletionHandler {
	return &CompletionHandler{service: completionService}
}

func (h *CompletionHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/captures/:captureId/completion/preview", h.Preview)
	group.POST("/captures/:captureId/completion/apply", h.Apply)
	group.POST("/captures/:captureId/completion/undo", h.Undo)
}

// Preview generates field proposals without mutating the capture or card.
func (h *CompletionHandler) Preview(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	captureID, ok := parseCaptureIDParam(c)
	if !ok {
		return
	}

	result, err := h.service.Preview(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, CompletionPreviewResponse{
		Preview:   result,
		RequestID: getRequestID(c),
	})
}

// Apply fills the empty fields referenced by the given proposals.
func (h *CompletionHandler) Apply(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	captureID, ok := parseCaptureIDParam(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMemoryBodyBytes)
	var input service.ApplyCompletionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"invalid request body",
			map[string]any{"field": "body"},
		)
		return
	}

	result, err := h.service.Apply(c.Request.Context(), userID, captureID, input)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, CompletionApplyResponse{
		Apply:     result,
		RequestID: getRequestID(c),
	})
}

// Undo reverts the most recent applied AI completion.
func (h *CompletionHandler) Undo(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	captureID, ok := parseCaptureIDParam(c)
	if !ok {
		return
	}

	result, err := h.service.Undo(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, CompletionUndoResponse{
		Undo:      result,
		RequestID: getRequestID(c),
	})
}

func (h *CompletionHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrCompletionDisabled):
		writeV3Error(c, http.StatusConflict, V3ErrorFeatureNotEnabled, "AI 补全未开启", nil)
	case errors.Is(err, service.ErrCompletionNotReady), errors.Is(err, service.ErrNothingToUndo):
		writeV3Error(c, http.StatusConflict, V3ErrorPreconditionFailed, err.Error(), nil)
	case errors.Is(err, service.ErrCompletionVersionConflict):
		writeV3Error(c, http.StatusConflict, V3ErrorVersionConflict, err.Error(), nil)
	case errors.Is(err, service.ErrMemoryNotFound):
		writeV3Error(c, http.StatusNotFound, V3ErrorNotFound, "memory not found", nil)
	case errors.Is(err, service.ErrInvalidCompletionInput):
		writeV3Error(c, http.StatusBadRequest, V3ErrorInvalidArgument, err.Error(), nil)
	case errors.Is(err, service.ErrCompletionLLM):
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "AI 生成失败，请稍后重试", nil)
	default:
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "internal server error", nil)
	}
}
