package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxEchoBodyBytes = 1 << 12 // 4 KiB

type echoUseCase interface {
	CurrentEcho(ctx context.Context, userID uuid.UUID) (*service.EchoCurrentResult, error)
	Feedback(ctx context.Context, userID uuid.UUID, echoID uuid.UUID, verdict entity.EchoFeedbackVerdict) (*service.EchoFeedbackResult, error)
	GetSettings(ctx context.Context, userID uuid.UUID) (*entity.UserEchoSettings, error)
	UpdateSettings(ctx context.Context, input service.UpdateEchoSettingsInput) (*entity.UserEchoSettings, error)
}

// EchoHandler exposes the current-echo protocol: reading the offered echo,
// submitting feedback, and managing the server-side echo settings.
type EchoHandler struct {
	service echoUseCase
}

type EchoReasonResponse struct {
	Code entity.EchoReasonCode `json:"code"`
	Text string                `json:"text"`
}

type EchoMemoryResponse struct {
	CaptureID   uuid.UUID  `json:"capture_id"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Summary     *string    `json:"summary,omitempty"`
	PrimaryType string     `json:"primary_type"`
	CapturedAt  *time.Time `json:"captured_at,omitempty"`
	IsPinned    bool       `json:"is_pinned"`
}

type EchoCardResponse struct {
	ID        uuid.UUID          `json:"id"`
	Status    entity.EchoStatus  `json:"status"`
	Reason    EchoReasonResponse `json:"reason"`
	Memory    EchoMemoryResponse `json:"memory"`
	CreatedAt time.Time          `json:"created_at"`
}

// EchoCurrentResponse is the GET /echoes/current envelope. echo is omitted when
// no echo is currently open (disabled, between windows, or no candidates).
type EchoCurrentResponse struct {
	Enabled     bool               `json:"enabled"`
	Cadence     entity.EchoCadence `json:"cadence"`
	Revision    int64              `json:"revision"`
	Echo        *EchoCardResponse  `json:"echo,omitempty"`
	NextDueAt   *time.Time         `json:"next_due_at,omitempty"`
	EmptyReason string             `json:"empty_reason,omitempty"`
	RequestID   string             `json:"request_id"`
}

type EchoSettingsResponse struct {
	Settings  *entity.UserEchoSettings `json:"settings"`
	RequestID string                   `json:"request_id"`
}

type EchoFeedbackRequest struct {
	Verdict entity.EchoFeedbackVerdict `json:"verdict"`
}

type EchoFeedbackResponse struct {
	EchoID    uuid.UUID          `json:"echo_id"`
	Status    entity.EchoStatus  `json:"status"`
	NextDueAt time.Time          `json:"next_due_at"`
	RequestID string             `json:"request_id"`
}

type UpdateEchoSettingsRequest struct {
	ExpectedRevision *int64              `json:"expected_revision"`
	Enabled          *bool               `json:"enabled"`
	Cadence          *entity.EchoCadence `json:"cadence"`
}

func NewEchoHandler(echoService echoUseCase) *EchoHandler {
	return &EchoHandler{service: echoService}
}

func (h *EchoHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/echoes/current", h.Current)
	group.POST("/echoes/:echoID/feedback", h.Feedback)
	group.GET("/users/me/echo-settings", h.GetSettings)
	group.PATCH("/users/me/echo-settings", h.UpdateSettings)
}

// Current returns the current echo for the authenticated user, creating one on
// demand when the cadence window has opened. Disabled / between-window /
// no-candidate states come back as explicit empty payloads (HTTP 200), never
// as errors.
func (h *EchoHandler) Current(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	result, err := h.service.CurrentEcho(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	response := EchoCurrentResponse{
		Enabled:     result.Enabled,
		Cadence:     result.Cadence,
		Revision:    result.Revision,
		NextDueAt:   result.NextDueAt,
		EmptyReason: result.EmptyReason,
		RequestID:   getRequestID(c),
	}
	if result.Echo != nil {
		card := &EchoCardResponse{
			ID:        result.Echo.EchoID,
			Status:    result.Echo.Status,
			Reason:    EchoReasonResponse{Code: result.Echo.Reason.Code, Text: result.Echo.Reason.Text},
			CreatedAt: result.Echo.CreatedAt,
			Memory: EchoMemoryResponse{
				CaptureID:   result.Echo.Memory.CaptureID,
				Kind:        result.Echo.Memory.Kind,
				Title:       result.Echo.Memory.Title,
				Summary:     result.Echo.Memory.Summary,
				PrimaryType: result.Echo.Memory.PrimaryType,
				CapturedAt:  result.Echo.Memory.CapturedAt,
				IsPinned:    result.Echo.Memory.IsPinned,
			},
		}
		response.Echo = card
	}
	c.JSON(http.StatusOK, response)
}

// Feedback records the user's verdict for the given echo. Only an open echo can
// transition; a missing/cross-user echo is 404 and a non-open echo is 409.
func (h *EchoHandler) Feedback(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	echoID, err := uuid.Parse(c.Param("echoID"))
	if err != nil || echoID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"echo_id is required",
			map[string]any{"field": "echoID"},
		)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxEchoBodyBytes)
	var request EchoFeedbackRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.Verdict == "" {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"verdict is required",
			map[string]any{"field": "verdict"},
		)
		return
	}

	result, err := h.service.Feedback(c.Request.Context(), userID, echoID, request.Verdict)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, EchoFeedbackResponse{
		EchoID:    result.EchoID,
		Status:    result.Status,
		NextDueAt: result.NextDueAt,
		RequestID: getRequestID(c),
	})
}

func (h *EchoHandler) GetSettings(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	settings, err := h.service.GetSettings(c.Request.Context(), userID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, EchoSettingsResponse{Settings: settings, RequestID: getRequestID(c)})
}

func (h *EchoHandler) UpdateSettings(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxEchoBodyBytes)
	var request UpdateEchoSettingsRequest
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
	if request.Enabled == nil && request.Cadence == nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"at least one settings field is required",
			map[string]any{"field": "body"},
		)
		return
	}

	updated, err := h.service.UpdateSettings(c.Request.Context(), service.UpdateEchoSettingsInput{
		UserID:           userID,
		ExpectedRevision: *request.ExpectedRevision,
		Enabled:          request.Enabled,
		Cadence:          request.Cadence,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, EchoSettingsResponse{Settings: updated, RequestID: getRequestID(c)})
}

func (h *EchoHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrEchoNotFound):
		writeV3Error(c, http.StatusNotFound, V3ErrorNotFound, err.Error(), nil)
	case errors.Is(err, repository.ErrEchoNotOpen):
		writeV3Error(c, http.StatusConflict, V3ErrorVersionConflict, err.Error(), nil)
	case errors.Is(err, service.ErrInvalidEchoSettings),
		errors.Is(err, service.ErrInvalidEchoFeedback):
		writeV3Error(c, http.StatusBadRequest, V3ErrorInvalidArgument, err.Error(), nil)
	case errors.Is(err, service.ErrEchoSettingsConflict):
		writeV3Error(c, http.StatusConflict, V3ErrorVersionConflict, err.Error(), nil)
	default:
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "internal server error", nil)
	}
}
