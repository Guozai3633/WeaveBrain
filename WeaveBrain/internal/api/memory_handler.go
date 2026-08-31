package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxMemoryBodyBytes = 1 << 20 // 1 MiB

type memoryUseCase interface {
	List(ctx context.Context, userID uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error)
	GetDetail(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.MemoryAggregate, error)
	Correct(ctx context.Context, userID uuid.UUID, captureID uuid.UUID, input service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error)
	AppendNote(ctx context.Context, userID uuid.UUID, captureID uuid.UUID, text string) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error)
	SetPinned(ctx context.Context, userID uuid.UUID, captureID uuid.UUID, pinned bool) (*entity.CaptureAggregate, error)
	Archive(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.CaptureAggregate, error)
	Delete(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.CaptureAggregate, error)
}

// MemoryHandler exposes the memory stream, detail and mutations.
type MemoryHandler struct {
	service memoryUseCase
}

type MemoryListResponse struct {
	Items      []*entity.MemoryListEntry `json:"items"`
	NextCursor *string                   `json:"next_cursor"`
	RequestID  string                    `json:"request_id"`
}

type MemoryDetailResponse struct {
	Capture    *entity.Capture              `json:"capture"`
	MemoryCard *entity.MemoryCard           `json:"memory_card"`
	Audio      *entity.AudioAsset           `json:"audio,omitempty"`
	Transcript *entity.TranscriptRevision   `json:"transcript,omitempty"`
	Revisions  []*entity.EnrichmentRevision `json:"revisions"`
	RequestID  string                       `json:"request_id"`
}

type MemoryMutationResponse struct {
	Capture    *entity.Capture             `json:"capture"`
	MemoryCard *entity.MemoryCard          `json:"memory_card"`
	Revision   *entity.EnrichmentRevision  `json:"revision,omitempty"`
	RequestID  string                      `json:"request_id"`
}

type AppendNoteRequest struct {
	Text string `json:"text"`
}

type SetPinnedRequest struct {
	Pinned bool `json:"pinned"`
}

func NewMemoryHandler(memoryService memoryUseCase) *MemoryHandler {
	return &MemoryHandler{service: memoryService}
}

func (h *MemoryHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/memories", h.List)
	group.GET("/memories/:captureId", h.GetDetail)
	group.PATCH("/memories/:captureId", h.Correct)
	group.POST("/memories/:captureId/notes", h.AppendNote)
	group.POST("/memories/:captureId/pin", h.SetPinned)
	group.POST("/memories/:captureId/archive", h.Archive)
	group.DELETE("/memories/:captureId", h.Delete)
}

func (h *MemoryHandler) List(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 50 {
			writeV3Error(
				c,
				http.StatusBadRequest,
				V3ErrorInvalidArgument,
				"limit must be an integer between 1 and 50",
				map[string]any{"field": "limit"},
			)
			return
		}
		limit = parsed
	}

	var pinnedOnly bool
	if raw := strings.TrimSpace(c.Query("pinned")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			writeV3Error(
				c,
				http.StatusBadRequest,
				V3ErrorInvalidArgument,
				"pinned must be true or false",
				map[string]any{"field": "pinned"},
			)
			return
		}
		pinnedOnly = parsed
	}

	q := entity.MemoryListQuery{
		Cursor:          nullableQuery(c, "cursor"),
		Limit:           limit,
		Q:               c.Query("q"),
		Kind:            c.Query("kind"),
		PrimaryType:     c.Query("primary_type"),
		LifecycleStatus: c.Query("lifecycle_status"),
		PinnedOnly:      pinnedOnly,
	}

	result, err := h.service.List(c.Request.Context(), userID, q)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemoryListResponse{
		Items:      result.Items,
		NextCursor: result.NextCursor,
		RequestID:  getRequestID(c),
	})
}

func (h *MemoryHandler) GetDetail(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	captureID, ok := parseCaptureIDParam(c)
	if !ok {
		return
	}

	detail, err := h.service.GetDetail(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemoryDetailResponse{
		Capture:    detail.Capture,
		MemoryCard: detail.MemoryCard,
		Audio:      detail.Audio,
		Transcript: detail.Transcript,
		Revisions:  detail.Revisions,
		RequestID:  getRequestID(c),
	})
}

func (h *MemoryHandler) Correct(c *gin.Context) {
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
	var request service.CorrectMemoryInput
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

	revision, aggregate, err := h.service.Correct(c.Request.Context(), userID, captureID, request)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemoryMutationResponse{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
		Revision:   revision,
		RequestID:  getRequestID(c),
	})
}

func (h *MemoryHandler) AppendNote(c *gin.Context) {
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
	var request AppendNoteRequest
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

	revision, aggregate, err := h.service.AppendNote(c.Request.Context(), userID, captureID, request.Text)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemoryMutationResponse{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
		Revision:   revision,
		RequestID:  getRequestID(c),
	})
}

func (h *MemoryHandler) SetPinned(c *gin.Context) {
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
	var request SetPinnedRequest
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

	aggregate, err := h.service.SetPinned(c.Request.Context(), userID, captureID, request.Pinned)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemoryMutationResponse{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
		RequestID:  getRequestID(c),
	})
}

func (h *MemoryHandler) Archive(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	captureID, ok := parseCaptureIDParam(c)
	if !ok {
		return
	}

	aggregate, err := h.service.Archive(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemoryMutationResponse{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
		RequestID:  getRequestID(c),
	})
}

func (h *MemoryHandler) Delete(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	captureID, ok := parseCaptureIDParam(c)
	if !ok {
		return
	}

	aggregate, err := h.service.Delete(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, MemoryMutationResponse{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
		RequestID:  getRequestID(c),
	})
}

func (h *MemoryHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidMemoryInput), errors.Is(err, service.ErrInvalidCursor):
		writeV3Error(c, http.StatusBadRequest, V3ErrorInvalidArgument, err.Error(), nil)
	case errors.Is(err, service.ErrMemoryNotFound):
		writeV3Error(c, http.StatusNotFound, V3ErrorNotFound, "memory not found", nil)
	case errors.Is(err, service.ErrMemoryVersionConflict):
		writeV3Error(c, http.StatusConflict, V3ErrorVersionConflict, err.Error(), nil)
	default:
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "internal server error", nil)
	}
}

func parseCaptureIDParam(c *gin.Context) (uuid.UUID, bool) {
	captureID, err := uuid.Parse(c.Param("captureId"))
	if err != nil || captureID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"capture id must be a valid UUID",
			map[string]any{"field": "captureId"},
		)
		return uuid.Nil, false
	}
	return captureID, true
}

func nullableQuery(c *gin.Context, key string) *string {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	return &value
}
