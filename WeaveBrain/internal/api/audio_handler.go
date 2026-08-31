package api

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	chunkSHA256Header     = "X-Chunk-SHA256"
	maxAudioRequestBody   = service.MaxAudioChunkBytes + 1024
)

type audioUseCase interface {
	Initiate(
		ctx context.Context,
		userID uuid.UUID,
		input service.InitiateAudioAssetInput,
	) (*entity.AudioAsset, error)
	GetByID(
		ctx context.Context,
		userID uuid.UUID,
		assetID uuid.UUID,
	) (*entity.AudioAsset, error)
	GetByCapture(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
	) (*entity.CaptureAudioAggregate, error)
	UploadChunk(
		ctx context.Context,
		userID uuid.UUID,
		assetID uuid.UUID,
		chunkIndex int32,
		declaredHash string,
		data []byte,
	) error
	Complete(
		ctx context.Context,
		userID uuid.UUID,
		assetID uuid.UUID,
	) (*entity.AudioAsset, error)
	TranscribeCapture(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
	) (*entity.TranscriptRevision, error)
	CorrectTranscript(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
		text string,
	) (*entity.TranscriptRevision, error)
	GetLatestTranscript(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
	) (*entity.TranscriptRevision, error)
}

type AudioAssetHandler struct {
	service audioUseCase
}

type InitiateAudioAssetRequest struct {
	AssetID     uuid.UUID `json:"asset_id"`
	CaptureID   uuid.UUID `json:"capture_id"`
	MimeType    string    `json:"mime_type"`
	DurationMs  *int64    `json:"duration_ms"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	TotalChunks int32     `json:"total_chunks"`
	STTEnabled  bool      `json:"stt_enabled"`
}

type UpdateTranscriptRequest struct {
	Text string `json:"text"`
}

type TranscriptResponse struct {
	Transcript *entity.TranscriptRevision `json:"transcript"`
	RequestID  string                     `json:"request_id"`
}

type AudioDetailResponse struct {
	Capture    *entity.Capture             `json:"capture"`
	MemoryCard *entity.MemoryCard          `json:"memory_card"`
	Audio      *entity.AudioAsset          `json:"audio,omitempty"`
	Transcript *entity.TranscriptRevision  `json:"transcript,omitempty"`
	RequestID  string                      `json:"request_id"`
}

func NewAudioAssetHandler(audioService audioUseCase) *AudioAssetHandler {
	return &AudioAssetHandler{service: audioService}
}

func (h *AudioAssetHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/audio-assets", h.Initiate)
	group.GET("/audio-assets/:assetID", h.GetByID)
	group.GET("/audio-assets/by-capture/:captureID", h.GetByCapture)
	group.PUT("/audio-assets/:assetID/chunks/:index", h.UploadChunk)
	group.POST("/audio-assets/:assetID/complete", h.Complete)
	group.POST("/captures/:captureID/transcribe", h.Transcribe)
	group.PATCH("/captures/:captureID/transcript", h.CorrectTranscript)
	group.GET("/captures/:captureID/transcript", h.GetTranscript)
}

func (h *AudioAssetHandler) Initiate(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	var request InitiateAudioAssetRequest
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

	asset, err := h.service.Initiate(c.Request.Context(), userID, service.InitiateAudioAssetInput{
		AssetID:     request.AssetID,
		CaptureID:   request.CaptureID,
		MimeType:    request.MimeType,
		DurationMs:  request.DurationMs,
		SizeBytes:   request.SizeBytes,
		SHA256:      request.SHA256,
		TotalChunks: request.TotalChunks,
		STTEnabled:  request.STTEnabled,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	c.JSON(http.StatusCreated, AudioAssetResponse{Asset: asset, RequestID: getRequestID(c)})
}

func (h *AudioAssetHandler) GetByID(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	assetID, err := uuid.Parse(c.Param("assetID"))
	if err != nil || assetID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"asset id must be a valid UUID",
			map[string]any{"field": "assetID"},
		)
		return
	}

	asset, err := h.service.GetByID(c.Request.Context(), userID, assetID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, AudioAssetResponse{Asset: asset, RequestID: getRequestID(c)})
}

func (h *AudioAssetHandler) GetByCapture(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	captureID, err := uuid.Parse(c.Param("captureID"))
	if err != nil || captureID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"capture id must be a valid UUID",
			map[string]any{"field": "captureID"},
		)
		return
	}

	agg, err := h.service.GetByCapture(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, AudioDetailResponse{
		Capture:    agg.Capture,
		MemoryCard: agg.MemoryCard,
		Audio:      agg.Audio,
		Transcript: agg.Transcript,
		RequestID:  getRequestID(c),
	})
}

func (h *AudioAssetHandler) UploadChunk(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	assetID, err := uuid.Parse(c.Param("assetID"))
	if err != nil || assetID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"asset id must be a valid UUID",
			map[string]any{"field": "assetID"},
		)
		return
	}

	var chunkIndex int32
	if _, err := fmt.Sscanf(c.Param("index"), "%d", &chunkIndex); err != nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"chunk index must be an integer",
			map[string]any{"field": "index"},
		)
		return
	}

	declaredHash := strings.TrimSpace(c.GetHeader(chunkSHA256Header))
	if _, err := hexDecodeSHA256(declaredHash); err != nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			chunkSHA256Header+" must be a 64-char lowercase hex SHA-256",
			map[string]any{"field": chunkSHA256Header},
		)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAudioRequestBody)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"failed to read request body",
			nil,
		)
		return
	}

	if err := h.service.UploadChunk(c.Request.Context(), userID, assetID, chunkIndex, declaredHash, data); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AudioAssetHandler) Complete(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	assetID, err := uuid.Parse(c.Param("assetID"))
	if err != nil || assetID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"asset id must be a valid UUID",
			map[string]any{"field": "assetID"},
		)
		return
	}

	asset, err := h.service.Complete(c.Request.Context(), userID, assetID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, AudioAssetResponse{Asset: asset, RequestID: getRequestID(c)})
}

func (h *AudioAssetHandler) Transcribe(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	captureID, err := uuid.Parse(c.Param("captureID"))
	if err != nil || captureID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"capture id must be a valid UUID",
			map[string]any{"field": "captureID"},
		)
		return
	}

	transcript, err := h.service.TranscribeCapture(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, TranscriptResponse{Transcript: transcript, RequestID: getRequestID(c)})
}

func (h *AudioAssetHandler) CorrectTranscript(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	captureID, err := uuid.Parse(c.Param("captureID"))
	if err != nil || captureID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"capture id must be a valid UUID",
			map[string]any{"field": "captureID"},
		)
		return
	}

	var request UpdateTranscriptRequest
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

	transcript, err := h.service.CorrectTranscript(c.Request.Context(), userID, captureID, request.Text)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, TranscriptResponse{Transcript: transcript, RequestID: getRequestID(c)})
}

func (h *AudioAssetHandler) GetTranscript(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}

	captureID, err := uuid.Parse(c.Param("captureID"))
	if err != nil || captureID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"capture id must be a valid UUID",
			map[string]any{"field": "captureID"},
		)
		return
	}

	transcript, err := h.service.GetLatestTranscript(c.Request.Context(), userID, captureID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, TranscriptResponse{Transcript: transcript, RequestID: getRequestID(c)})
}

func (h *AudioAssetHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidAudioAsset),
		errors.Is(err, service.ErrInvalidTranscript),
		errors.Is(err, service.ErrAudioChecksumMismatch),
		errors.Is(err, service.ErrAudioChunkOutOfRange),
		errors.Is(err, service.ErrAudioChunkTooLarge):
		writeV3Error(c, http.StatusBadRequest, V3ErrorInvalidArgument, err.Error(), nil)
	case errors.Is(err, service.ErrAudioAssetNotFound),
		errors.Is(err, service.ErrCaptureNotFound):
		writeV3Error(c, http.StatusNotFound, V3ErrorNotFound, "resource not found", nil)
	case errors.Is(err, service.ErrAudioUploadNotReady):
		writeV3Error(c, http.StatusPreconditionFailed, V3ErrorPreconditionFailed, err.Error(), nil)
	default:
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "internal server error", nil)
	}
}

type AudioAssetResponse struct {
	Asset     *entity.AudioAsset `json:"asset"`
	RequestID string             `json:"request_id"`
}

func hexDecodeSHA256(s string) ([]byte, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("invalid sha256")
	}
	return decoded, nil
}
