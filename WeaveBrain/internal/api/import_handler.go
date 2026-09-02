package api

import (
	"context"
	"encoding/csv"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxImportRequestBodyBytes = 8 << 20 // 8 MiB MVP budget

// importUseCase is the service surface the batch-import handler needs.
type importUseCase interface {
	CreateJob(ctx context.Context, userID uuid.UUID, input service.CreateImportJobInput) (*entity.ImportJob, error)
	GetJob(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error)
	GetPreview(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, limit, offset int) (*entity.ImportPreviewResult, error)
	CompletionPreview(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, rowNumbers []int) (*entity.ImportCompletionResult, error)
	CompletionApply(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, selections []service.ImportRowSelection) (*entity.ImportCompletionResult, error)
	Commit(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, input service.ImportCommitInput) (*entity.ImportCommitResult, error)
	GetErrorReport(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) ([]entity.ImportErrorReportEntry, error)
	Cancel(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error)
}

// ImportHandler exposes the batch import flow: parse → preview → AI completion
// → commit, plus the downloadable error report.
type ImportHandler struct {
	service importUseCase
}

type CreateImportJobRequest struct {
	Format           entity.ImportFormat `json:"format"`
	SourceName       string              `json:"source_name"`
	Content          string              `json:"content"`
	Separator        string              `json:"separator"`
	Timezone         *string             `json:"timezone"`
	OriginalFilename *string             `json:"original_filename"`
	// PrivacyMode is accepted for forward compatibility; per-row privacy flows
	// through the capture pipeline default in this round.
	PrivacyMode string `json:"privacy_mode"`
}

type ImportJobResponse struct {
	Job       *entity.ImportJob `json:"job"`
	RequestID string            `json:"request_id"`
}

type ImportPreviewResponse struct {
	Preview   *entity.ImportPreviewResult `json:"preview"`
	RequestID string                      `json:"request_id"`
}

type ImportCompletionPreviewRequest struct {
	RowNumbers []int `json:"row_numbers"`
}

type ImportRowSelectionRequest struct {
	RowNumber   int         `json:"row_number"`
	ProposalIDs []uuid.UUID `json:"proposal_ids"`
}

type ImportCompletionApplyRequest struct {
	RowSelections []ImportRowSelectionRequest `json:"row_selections"`
}

type ImportCompletionResponse struct {
	Completion *entity.ImportCompletionResult `json:"completion"`
	RequestID  string                         `json:"request_id"`
}

type ImportCommitRequest struct {
	DuplicateContentAction string         `json:"duplicate_content_action"`
	RowActions             map[int]string `json:"row_actions"`
	// SkipNeedsInput is accepted for forward compatibility; rows that need
	// input are always skipped and reported in this round.
	SkipNeedsInput *bool `json:"skip_needs_input"`
}

type ImportCommitResponse struct {
	Commit    *entity.ImportCommitResult `json:"commit"`
	RequestID string                     `json:"request_id"`
}

type ImportErrorReportResponse struct {
	Entries   []entity.ImportErrorReportEntry `json:"entries"`
	RequestID string                          `json:"request_id"`
}

func NewImportHandler(importService importUseCase) *ImportHandler {
	return &ImportHandler{service: importService}
}

func (h *ImportHandler) RegisterRoutes(group *gin.RouterGroup) {
	group.POST("/imports", h.CreateJob)
	group.GET("/imports/:id", h.GetJob)
	group.GET("/imports/:id/preview", h.GetPreview)
	group.POST("/imports/:id/completion/preview", h.CompletionPreview)
	group.POST("/imports/:id/completion/apply", h.CompletionApply)
	group.POST("/imports/:id/commit", h.Commit)
	group.GET("/imports/:id/error-report", h.GetErrorReport)
	group.POST("/imports/:id/cancel", h.Cancel)
}

// normalizeImportFormat maps the file-extension aliases a client may send
// (txt / markdown) onto the plain_text parser.
func normalizeImportFormat(raw string) entity.ImportFormat {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "txt", "text", "markdown", "md":
		return entity.ImportFormatPlainText
	default:
		return entity.ImportFormat(raw)
	}
}

// CreateJob parses the submitted text into a draft import job.
func (h *ImportHandler) CreateJob(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportRequestBodyBytes)

	var request CreateImportJobRequest
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

	job, err := h.service.CreateJob(c.Request.Context(), userID, service.CreateImportJobInput{
		Format:           normalizeImportFormat(string(request.Format)),
		SourceName:       request.SourceName,
		Content:          request.Content,
		Separator:        request.Separator,
		Timezone:         request.Timezone,
		OriginalFilename: request.OriginalFilename,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, ImportJobResponse{Job: job, RequestID: getRequestID(c)})
}

// GetJob returns one import job owned by the user.
func (h *ImportHandler) GetJob(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	jobID, ok := parseImportJobIDParam(c)
	if !ok {
		return
	}

	job, err := h.service.GetJob(c.Request.Context(), userID, jobID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ImportJobResponse{Job: job, RequestID: getRequestID(c)})
}

// GetPreview returns a page of rows (default first 10) plus the column mapping.
func (h *ImportHandler) GetPreview(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	jobID, ok := parseImportJobIDParam(c)
	if !ok {
		return
	}

	limit := 10
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeV3Error(
				c,
				http.StatusBadRequest,
				V3ErrorInvalidArgument,
				"limit must be a positive integer",
				map[string]any{"field": "limit"},
			)
			return
		}
		limit = parsed
	}
	offset := 0
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeV3Error(
				c,
				http.StatusBadRequest,
				V3ErrorInvalidArgument,
				"offset must be a non-negative integer",
				map[string]any{"field": "offset"},
			)
			return
		}
		offset = parsed
	}

	preview, err := h.service.GetPreview(c.Request.Context(), userID, jobID, limit, offset)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ImportPreviewResponse{Preview: preview, RequestID: getRequestID(c)})
}

// CompletionPreview generates AI field proposals for the selected rows (all
// rows when row_numbers is absent). A single row's failure is recorded as a
// completion_error and never blocks the other rows.
func (h *ImportHandler) CompletionPreview(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	jobID, ok := parseImportJobIDParam(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportRequestBodyBytes)
	var request ImportCompletionPreviewRequest
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

	result, err := h.service.CompletionPreview(c.Request.Context(), userID, jobID, request.RowNumbers)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ImportCompletionResponse{Completion: result, RequestID: getRequestID(c)})
}

// CompletionApply marks the selected proposal IDs accepted for each row.
func (h *ImportHandler) CompletionApply(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	jobID, ok := parseImportJobIDParam(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportRequestBodyBytes)
	var request ImportCompletionApplyRequest
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

	selections := make([]service.ImportRowSelection, 0, len(request.RowSelections))
	for _, sel := range request.RowSelections {
		selections = append(selections, service.ImportRowSelection{
			RowNumber:   sel.RowNumber,
			ProposalIDs: sel.ProposalIDs,
		})
	}

	result, err := h.service.CompletionApply(c.Request.Context(), userID, jobID, selections)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ImportCompletionResponse{Completion: result, RequestID: getRequestID(c)})
}

// Commit imports the job's rows, one transaction per row.
func (h *ImportHandler) Commit(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	jobID, ok := parseImportJobIDParam(c)
	if !ok {
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportRequestBodyBytes)
	var request ImportCommitRequest
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

	result, err := h.service.Commit(c.Request.Context(), userID, jobID, service.ImportCommitInput{
		DuplicateContentAction: request.DuplicateContentAction,
		RowActions:             request.RowActions,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ImportCommitResponse{Commit: result, RequestID: getRequestID(c)})
}

// GetErrorReport returns the rows that did not import, as CSV or JSON.
func (h *ImportHandler) GetErrorReport(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	jobID, ok := parseImportJobIDParam(c)
	if !ok {
		return
	}

	entries, err := h.service.GetErrorReport(c.Request.Context(), userID, jobID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}

	if strings.EqualFold(strings.TrimSpace(c.Query("format")), "csv") {
		writeErrorReportCSV(c, entries)
		return
	}
	c.JSON(http.StatusOK, ImportErrorReportResponse{Entries: entries, RequestID: getRequestID(c)})
}

// Cancel marks a non-completed job cancelled.
func (h *ImportHandler) Cancel(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok || userID == uuid.Nil {
		writeV3Error(c, http.StatusUnauthorized, V3ErrorUnauthorized, "authentication required", nil)
		return
	}
	jobID, ok := parseImportJobIDParam(c)
	if !ok {
		return
	}

	job, err := h.service.Cancel(c.Request.Context(), userID, jobID)
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, ImportJobResponse{Job: job, RequestID: getRequestID(c)})
}

func (h *ImportHandler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrImportInvalid):
		writeV3Error(c, http.StatusBadRequest, V3ErrorInvalidArgument, err.Error(), nil)
	case errors.Is(err, service.ErrImportNotFound):
		writeV3Error(c, http.StatusNotFound, V3ErrorNotFound, "import job not found", nil)
	case errors.Is(err, service.ErrImportAlreadyCompleted):
		writeV3Error(c, http.StatusConflict, V3ErrorPreconditionFailed, err.Error(), nil)
	case errors.Is(err, service.ErrImportCompletionDisabled):
		writeV3Error(c, http.StatusConflict, V3ErrorFeatureNotEnabled, "AI 补全未开启", nil)
	case errors.Is(err, service.ErrImportCompletionLLM):
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "AI 生成失败，请稍后重试", nil)
	default:
		writeV3Error(c, http.StatusInternalServerError, V3ErrorInternal, "internal server error", nil)
	}
}

func parseImportJobIDParam(c *gin.Context) (uuid.UUID, bool) {
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil || jobID == uuid.Nil {
		writeV3Error(
			c,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			"import id must be a valid UUID",
			map[string]any{"field": "id"},
		)
		return uuid.Nil, false
	}
	return jobID, true
}

func writeErrorReportCSV(c *gin.Context, entries []entity.ImportErrorReportEntry) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="import_error_report.csv"`)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"row_number", "external_id", "status", "dedupe_status", "error_code", "error_message"})
	for _, entry := range entries {
		externalID := ""
		if entry.ExternalID != nil {
			externalID = *entry.ExternalID
		}
		codes := make([]string, 0, len(entry.ValidationErrors))
		messages := make([]string, 0, len(entry.ValidationErrors))
		for _, ve := range entry.ValidationErrors {
			codes = append(codes, ve.Code)
			messages = append(messages, ve.Message)
		}
		_ = w.Write([]string{
			strconv.Itoa(entry.RowNumber),
			externalID,
			string(entry.Status),
			string(entry.DedupeStatus),
			strings.Join(codes, ";"),
			strings.Join(messages, ";"),
		})
	}
	w.Flush()
}
