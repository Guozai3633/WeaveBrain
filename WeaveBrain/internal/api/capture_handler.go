package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	idempotencyKeyHeader       = "Idempotency-Key"
	maxCaptureRequestBodyBytes = 1 << 20
)

type captureUseCase interface {
	Create(
		ctx context.Context,
		userID uuid.UUID,
		input service.CreateCaptureInput,
	) (*service.CreateCaptureResult, error)
	GetByID(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
	) (*entity.CaptureAggregate, error)
}

type CaptureHandler struct {
	service captureUseCase
}

type CreateCaptureRequest struct {
	CaptureID           uuid.UUID          `json:"capture_id"`
	Kind                entity.CaptureKind `json:"kind"`
	Text                string             `json:"text"`
	CapturedAt          *time.Time         `json:"captured_at"`
	CapturedAtPrecision string             `json:"captured_at_precision"`
	Timezone            *string            `json:"timezone"`
	Source              string             `json:"source"`
	CollectionID        *int64             `json:"collection_id"`
	PrivacyMode         string             `json:"privacy_mode"`
	ClientVersion       int                `json:"client_version"`
	// Import passthrough (kind=import): ExternalID + SourceName form the dedup
	// key; Title/Tags/PrimaryType seed the initial MemoryCard.
	ExternalID  *string  `json:"external_id"`
	SourceName  *string  `json:"source_name"`
	Title       *string  `json:"title"`
	Tags        []string `json:"tags"`
	PrimaryType *string  `json:"primary_type"`
}

type CaptureResponse struct {
	Capture    *entity.Capture      `json:"capture"`
	MemoryCard *entity.MemoryCard   `json:"memory_card"`
	Dedupe     *entity.CaptureDedupe `json:"dedupe,omitempty"`
	Replayed   bool                 `json:"replayed,omitempty"`
	RequestID  string               `json:"request_id"`
}

func NewCaptureHandler(captureService captureUseCase) *CaptureHandler {
	return &CaptureHandler{service: captureService}
}

func (h *CaptureHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/captures", h.Create)
	group.GET("/captures/:id", h.GetByID)
}

func (h *CaptureHandler) Create(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	idempotencyKey := strings.TrimSpace(c.GetHeader(idempotencyKeyHeader))
	keyID, err := uuid.Parse(idempotencyKey)
	if idempotencyKey == "" || err != nil || keyID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"Idempotency-Key must be a valid UUID",
			map[string]any{"field": idempotencyKeyHeader},
		)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCaptureRequestBodyBytes)

	var request CreateCaptureRequest
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
	if request.CaptureID != keyID {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"capture_id must match Idempotency-Key",
			map[string]any{"field": "capture_id"},
		)
		return
	}

	result, err := h.service.Create(c.Request.Context(), userID, service.CreateCaptureInput{
		ID:                  request.CaptureID,
		Kind:                request.Kind,
		Text:                request.Text,
		CapturedAt:          request.CapturedAt,
		CapturedAtPrecision: request.CapturedAtPrecision,
		Timezone:            request.Timezone,
		Source:              request.Source,
		CollectionID:        request.CollectionID,
		PrivacyMode:         request.PrivacyMode,
		ClientVersion:       request.ClientVersion,
		ExternalID:          request.ExternalID,
		SourceName:          request.SourceName,
		TitleOverride:       request.Title,
		TagsOverride:        request.Tags,
		PrimaryTypeOverride: request.PrimaryType,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	c.JSON(status, CaptureResponse{
		Capture:    result.Aggregate.Capture,
		MemoryCard: result.Aggregate.MemoryCard,
		Dedupe:     result.Dedupe,
		Replayed:   result.Replayed,
		RequestID:  getRequestID(c),
	})
}

func (h *CaptureHandler) GetByID(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	captureID, err := uuid.Parse(c.Param("id"))
	if err != nil || captureID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"capture id must be a valid UUID",
			map[string]any{"field": "id"},
		)
		return
	}

	aggregate, err := h.service.GetByID(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, CaptureResponse{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
		RequestID:  getRequestID(c),
	})
}

func (h *CaptureHandler) writeServiceError(c *gin.Context, err error) {
	var dupErr *service.DuplicateExternalIDError
	switch {
	case errors.Is(err, service.ErrInvalidCapture):
		writeV3Error(c, http.StatusBadRequest, V3ErrorInvalidArgument, err.Error(), nil)
	case errors.Is(err, service.ErrCaptureNotFound):
		writeV3Error(c, http.StatusNotFound, V3ErrorNotFound, "capture not found", nil)
	case errors.Is(err, service.ErrCaptureIdempotencyConflict):
		writeV3Error(
			c,
			http.StatusConflict,
			V3ErrorIdempotencyConflict,
			"Idempotency-Key was already used with different content",
			nil,
		)
	case errors.As(err, &dupErr):
		writeV3Error(
			c,
			http.StatusConflict,
			V3ErrorPreconditionFailed,
			"external_id 已导入过",
			map[string]any{"existing_capture_id": dupErr.ExistingCaptureID},
		)
	default:
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "internal server error", nil)
	}
}
