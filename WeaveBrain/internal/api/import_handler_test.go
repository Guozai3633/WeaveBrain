package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeImportUseCase struct {
	createJobFn        func(context.Context, uuid.UUID, service.CreateImportJobInput) (*entity.ImportJob, error)
	getJobFn           func(context.Context, uuid.UUID, uuid.UUID) (*entity.ImportJob, error)
	getPreviewFn       func(context.Context, uuid.UUID, uuid.UUID, int, int) (*entity.ImportPreviewResult, error)
	completionPreviewFn func(context.Context, uuid.UUID, uuid.UUID, []int) (*entity.ImportCompletionResult, error)
	completionApplyFn  func(context.Context, uuid.UUID, uuid.UUID, []service.ImportRowSelection) (*entity.ImportCompletionResult, error)
	commitFn           func(context.Context, uuid.UUID, uuid.UUID, service.ImportCommitInput) (*entity.ImportCommitResult, error)
	getErrorReportFn   func(context.Context, uuid.UUID, uuid.UUID) ([]entity.ImportErrorReportEntry, error)
	cancelFn           func(context.Context, uuid.UUID, uuid.UUID) (*entity.ImportJob, error)
}

func (f *fakeImportUseCase) CreateJob(ctx context.Context, userID uuid.UUID, input service.CreateImportJobInput) (*entity.ImportJob, error) {
	if f.createJobFn == nil {
		return importTestJob(userID), nil
	}
	return f.createJobFn(ctx, userID, input)
}

func (f *fakeImportUseCase) GetJob(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error) {
	if f.getJobFn == nil {
		return importTestJob(userID), nil
	}
	return f.getJobFn(ctx, userID, jobID)
}

func (f *fakeImportUseCase) GetPreview(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, limit, offset int) (*entity.ImportPreviewResult, error) {
	if f.getPreviewFn == nil {
		return &entity.ImportPreviewResult{Job: importTestJob(userID), Rows: []*entity.ImportRow{}}, nil
	}
	return f.getPreviewFn(ctx, userID, jobID, limit, offset)
}

func (f *fakeImportUseCase) CompletionPreview(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, rowNumbers []int) (*entity.ImportCompletionResult, error) {
	if f.completionPreviewFn == nil {
		return &entity.ImportCompletionResult{Rows: []*entity.ImportRow{importTestRow(userID, jobID, 1)}}, nil
	}
	return f.completionPreviewFn(ctx, userID, jobID, rowNumbers)
}

func (f *fakeImportUseCase) CompletionApply(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, selections []service.ImportRowSelection) (*entity.ImportCompletionResult, error) {
	if f.completionApplyFn == nil {
		return &entity.ImportCompletionResult{Rows: []*entity.ImportRow{importTestRow(userID, jobID, 1)}}, nil
	}
	return f.completionApplyFn(ctx, userID, jobID, selections)
}

func (f *fakeImportUseCase) Commit(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, input service.ImportCommitInput) (*entity.ImportCommitResult, error) {
	if f.commitFn == nil {
		return &entity.ImportCommitResult{Imported: 1, Total: 1, JobStatus: entity.ImportJobStatusCompleted}, nil
	}
	return f.commitFn(ctx, userID, jobID, input)
}

func (f *fakeImportUseCase) GetErrorReport(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) ([]entity.ImportErrorReportEntry, error) {
	if f.getErrorReportFn == nil {
		return nil, nil
	}
	return f.getErrorReportFn(ctx, userID, jobID)
}

func (f *fakeImportUseCase) Cancel(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error) {
	if f.cancelFn == nil {
		return importTestJob(userID), nil
	}
	return f.cancelFn(ctx, userID, jobID)
}

func stringPtr(s string) *string { return &s }

func importTestJob(userID uuid.UUID) *entity.ImportJob {
	now := time.Now().UTC()
	return &entity.ImportJob{
		ID:            uuid.New(),
		UserID:        userID,
		SourceName:    "旧备忘录",
		Format:        entity.ImportFormatPlainText,
		ColumnMapping: map[string]any{"content": "content"},
		TotalRows:     1,
		ValidRows:     1,
		Status:        entity.ImportJobStatusDraft,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func importTestRow(userID, jobID uuid.UUID, rowNumber int) *entity.ImportRow {
	now := time.Now().UTC()
	return &entity.ImportRow{
		ID:            uuid.New(),
		ImportJobID:   jobID,
		UserID:        userID,
		RowNumber:     rowNumber,
		Content:       stringPtr("内容"),
		DedupeStatus:  entity.ImportDedupeNone,
		Status:        entity.ImportRowStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func newImportHandlerTestEngine(userID uuid.UUID, useCase importUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		SetCurrentUserID(c, userID)
		c.Next()
	})
	NewImportHandler(useCase).RegisterRoutes(group)
	return engine
}

func TestImportHandlerCreateJobReturns201(t *testing.T) {
	userID := uuid.New()
	var got service.CreateImportJobInput
	useCase := &fakeImportUseCase{
		createJobFn: func(_ context.Context, u uuid.UUID, input service.CreateImportJobInput) (*entity.ImportJob, error) {
			got = input
			return importTestJob(userID), nil
		},
	}
	engine := newImportHandlerTestEngine(userID, useCase)

	body := `{"format":"markdown","source_name":"旧笔记","content":"# 标题\n\n正文","timezone":"Asia/Shanghai"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v3/imports", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got.Format != entity.ImportFormatPlainText {
		t.Fatalf("expected markdown alias mapped to plain_text, got %s", got.Format)
	}
	if got.SourceName != "旧笔记" || got.Timezone == nil || *got.Timezone != "Asia/Shanghai" {
		t.Fatalf("unexpected input: %#v", got)
	}
	var resp ImportJobResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Job == nil || resp.Job.SourceName != "旧备忘录" {
		t.Fatalf("unexpected job response: %#v", resp)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestImportHandlerCreateJobInvalidBody(t *testing.T) {
	userID := uuid.New()
	engine := newImportHandlerTestEngine(userID, &fakeImportUseCase{})
	for _, body := range []string{"{", `{"format":123}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v3/imports", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		assertV3ErrorResponse(
			t,
			recorder,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			recorder.Header().Get(requestIDHeader),
		)
	}
}

func TestImportHandlerGetJobAndPreview(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()
	job := importTestJob(userID)
	job.ID = jobID
	useCase := &fakeImportUseCase{
		getJobFn: func(_ context.Context, u uuid.UUID, got uuid.UUID) (*entity.ImportJob, error) {
			if got != jobID {
				t.Fatalf("getJob called with %v, want %v", got, jobID)
			}
			return job, nil
		},
		getPreviewFn: func(_ context.Context, u uuid.UUID, got uuid.UUID, limit, offset int) (*entity.ImportPreviewResult, error) {
			if got != jobID || limit != 10 || offset != 0 {
				t.Fatalf("unexpected preview args: id=%v limit=%d offset=%d", got, limit, offset)
			}
			return &entity.ImportPreviewResult{
				Job:         job,
				Rows:        []*entity.ImportRow{importTestRow(userID, jobID, 1)},
				NextCursor:  nil,
			}, nil
		},
	}
	engine := newImportHandlerTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/imports/"+jobID.String(), nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("get job expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v3/imports/"+jobID.String()+"/preview", nil)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("preview expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var previewResp ImportPreviewResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &previewResp); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if previewResp.Preview == nil || len(previewResp.Preview.Rows) != 1 {
		t.Fatalf("unexpected preview: %#v", previewResp.Preview)
	}
}

func TestImportHandlerCompletionPreviewAndApply(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()
	proposalID := uuid.New()
	var gotPreviewRows []int
	var gotSelections []service.ImportRowSelection
	useCase := &fakeImportUseCase{
		completionPreviewFn: func(_ context.Context, u uuid.UUID, got uuid.UUID, rows []int) (*entity.ImportCompletionResult, error) {
			if got != jobID {
				t.Fatalf("preview called with %v, want %v", got, jobID)
			}
			gotPreviewRows = rows
			row := importTestRow(userID, jobID, 1)
			row.CompletionProposals = []entity.ImportFieldProposal{{
				ID:            proposalID,
				FieldName:     "tags",
				ProposedValue: "[\"工作\"]",
				Provenance:    entity.ProvenanceAI,
				Status:        entity.ProposalStatusPending,
			}}
			return &entity.ImportCompletionResult{Rows: []*entity.ImportRow{row}}, nil
		},
		completionApplyFn: func(_ context.Context, u uuid.UUID, got uuid.UUID, sel []service.ImportRowSelection) (*entity.ImportCompletionResult, error) {
			gotSelections = sel
			row := importTestRow(userID, jobID, 1)
			row.CompletionProposals = []entity.ImportFieldProposal{{
				ID:            proposalID,
				FieldName:     "tags",
				ProposedValue: "[\"工作\"]",
				Provenance:    entity.ProvenanceAI,
				Status:        entity.ProposalStatusAccepted,
			}}
			return &entity.ImportCompletionResult{Rows: []*entity.ImportRow{row}}, nil
		},
	}
	engine := newImportHandlerTestEngine(userID, useCase)

	body := `{"row_numbers":[1]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v3/imports/"+jobID.String()+"/completion/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("completion preview expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(gotPreviewRows) != 1 || gotPreviewRows[0] != 1 {
		t.Fatalf("expected row_numbers [1], got %#v", gotPreviewRows)
	}
	var previewResp ImportCompletionResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &previewResp); err != nil {
		t.Fatalf("decode completion preview: %v", err)
	}
	if previewResp.Completion == nil || len(previewResp.Completion.Rows) != 1 {
		t.Fatalf("unexpected completion: %#v", previewResp.Completion)
	}

	body = `{"row_selections":[{"row_number":1,"proposal_ids":["` + proposalID.String() + `"]}]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v3/imports/"+jobID.String()+"/completion/apply", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("completion apply expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(gotSelections) != 1 || gotSelections[0].RowNumber != 1 || len(gotSelections[0].ProposalIDs) != 1 || gotSelections[0].ProposalIDs[0] != proposalID {
		t.Fatalf("unexpected selections: %#v", gotSelections)
	}
}

func TestImportHandlerCommit(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()
	var got service.ImportCommitInput
	useCase := &fakeImportUseCase{
		commitFn: func(_ context.Context, u uuid.UUID, gotID uuid.UUID, input service.ImportCommitInput) (*entity.ImportCommitResult, error) {
			if gotID != jobID {
				t.Fatalf("commit called with %v, want %v", gotID, jobID)
			}
			got = input
			return &entity.ImportCommitResult{
				Imported:   1,
				Skipped:    0,
				Failed:     0,
				NeedsInput: 0,
				Total:      1,
				JobStatus:  entity.ImportJobStatusCompleted,
			}, nil
		},
	}
	engine := newImportHandlerTestEngine(userID, useCase)

	body := `{"duplicate_content_action":"skip","row_actions":{"2":"import"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v3/imports/"+jobID.String()+"/commit", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("commit expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if got.DuplicateContentAction != "skip" || got.RowActions[2] != "import" {
		t.Fatalf("unexpected commit input: %#v", got)
	}
	var resp ImportCommitResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode commit: %v", err)
	}
	if resp.Commit == nil || resp.Commit.Imported != 1 {
		t.Fatalf("unexpected commit result: %#v", resp.Commit)
	}
}

func TestImportHandlerErrorReportCSV(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()
	useCase := &fakeImportUseCase{
		getErrorReportFn: func(_ context.Context, u uuid.UUID, got uuid.UUID) ([]entity.ImportErrorReportEntry, error) {
			if got != jobID {
				t.Fatalf("error-report called with %v, want %v", got, jobID)
			}
			externalID := "t1"
			return []entity.ImportErrorReportEntry{{
				RowNumber:    1,
				ExternalID:   &externalID,
				Status:       entity.ImportRowStatusSkipped,
				DedupeStatus: entity.ImportDedupeDuplicateExternal,
				ValidationErrors: []entity.ImportValidationError{{
					Code:    "duplicate_external",
					Message: "已导入过",
				}},
			}}, nil
		},
	}
	engine := newImportHandlerTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/imports/"+jobID.String()+"/error-report?format=csv", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if ct := recorder.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Fatalf("expected text/csv content type, got %q", ct)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "row_number") || !strings.Contains(body, "t1") || !strings.Contains(body, "duplicate_external") {
		t.Fatalf("unexpected csv body: %s", body)
	}
}

func TestImportHandlerErrorReportJSON(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()
	engine := newImportHandlerTestEngine(userID, &fakeImportUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/api/v3/imports/"+jobID.String()+"/error-report", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp ImportErrorReportResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestImportHandlerCancel(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()
	useCase := &fakeImportUseCase{
		cancelFn: func(_ context.Context, u uuid.UUID, got uuid.UUID) (*entity.ImportJob, error) {
			if got != jobID {
				t.Fatalf("cancel called with %v, want %v", got, jobID)
			}
			job := importTestJob(userID)
			job.Status = entity.ImportJobStatusCancelled
			return job, nil
		},
	}
	engine := newImportHandlerTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/imports/"+jobID.String()+"/cancel", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("cancel expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp ImportJobResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Job == nil || resp.Job.Status != entity.ImportJobStatusCancelled {
		t.Fatalf("unexpected cancel response: %#v", resp.Job)
	}
}

func TestImportHandlerRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) { c.Next() }) // no user
	NewImportHandler(&fakeImportUseCase{}).RegisterRoutes(group)

	jobID := uuid.NewString()
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v3/imports", `{"format":"plain_text","source_name":"s","content":"x"}`},
		{http.MethodGet, "/api/v3/imports/" + jobID, ``},
		{http.MethodGet, "/api/v3/imports/" + jobID + "/preview", ``},
		{http.MethodPost, "/api/v3/imports/" + jobID + "/completion/preview", `{}`},
		{http.MethodPost, "/api/v3/imports/" + jobID + "/completion/apply", `{"row_selections":[]}`},
		{http.MethodPost, "/api/v3/imports/" + jobID + "/commit", `{}`},
		{http.MethodGet, "/api/v3/imports/" + jobID + "/error-report", ``},
		{http.MethodPost, "/api/v3/imports/" + jobID + "/cancel", ``},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		assertV3ErrorResponse(
			t,
			recorder,
			http.StatusUnauthorized,
			V3ErrorUnauthorized,
			recorder.Header().Get(requestIDHeader),
		)
	}
}

func TestImportHandlerInvalidJobID(t *testing.T) {
	userID := uuid.New()
	engine := newImportHandlerTestEngine(userID, &fakeImportUseCase{})

	for _, path := range []string{
		"/api/v3/imports/not-a-uuid",
		"/api/v3/imports/not-a-uuid/preview",
		"/api/v3/imports/not-a-uuid/commit",
		"/api/v3/imports/not-a-uuid/cancel",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if strings.Contains(path, "/commit") || strings.Contains(path, "/cancel") {
			req = httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
		}
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)
		assertV3ErrorResponse(
			t,
			recorder,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			recorder.Header().Get(requestIDHeader),
		)
	}
}

func TestImportHandlerErrorMappings(t *testing.T) {
	userID := uuid.New()
	jobID := uuid.New()

	createErr := func(err error) *fakeImportUseCase {
		return &fakeImportUseCase{createJobFn: func(context.Context, uuid.UUID, service.CreateImportJobInput) (*entity.ImportJob, error) {
			return nil, err
		}}
	}
	getErr := func(err error) *fakeImportUseCase {
		return &fakeImportUseCase{getJobFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.ImportJob, error) {
			return nil, err
		}}
	}
	previewErr := func(err error) *fakeImportUseCase {
		return &fakeImportUseCase{completionPreviewFn: func(context.Context, uuid.UUID, uuid.UUID, []int) (*entity.ImportCompletionResult, error) {
			return nil, err
		}}
	}
	commitErr := func(err error) *fakeImportUseCase {
		return &fakeImportUseCase{commitFn: func(context.Context, uuid.UUID, uuid.UUID, service.ImportCommitInput) (*entity.ImportCommitResult, error) {
			return nil, err
		}}
	}

	cases := []struct {
		name           string
		useCase        *fakeImportUseCase
		method         string
		path           string
		body           string
		expectedStatus int
		expectedCode   V3ErrorCode
	}{
		{"create invalid", createErr(service.ErrImportInvalid), http.MethodPost, "/api/v3/imports", `{"format":"plain_text","source_name":"s","content":"x"}`, http.StatusBadRequest, V3ErrorInvalidArgument},
		{"get not found", getErr(service.ErrImportNotFound), http.MethodGet, "/api/v3/imports/" + jobID.String(), ``, http.StatusNotFound, V3ErrorNotFound},
		{"completion disabled", previewErr(service.ErrImportCompletionDisabled), http.MethodPost, "/api/v3/imports/" + jobID.String() + "/completion/preview", `{}`, http.StatusConflict, V3ErrorFeatureNotEnabled},
		{"completion llm", previewErr(service.ErrImportCompletionLLM), http.MethodPost, "/api/v3/imports/" + jobID.String() + "/completion/preview", `{}`, http.StatusInternalServerError, V3ErrorInternal},
		{"commit already completed", commitErr(service.ErrImportAlreadyCompleted), http.MethodPost, "/api/v3/imports/" + jobID.String() + "/commit", `{}`, http.StatusConflict, V3ErrorPreconditionFailed},
		{"commit invalid", commitErr(service.ErrImportInvalid), http.MethodPost, "/api/v3/imports/" + jobID.String() + "/commit", `{}`, http.StatusBadRequest, V3ErrorInvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := newImportHandlerTestEngine(userID, tc.useCase)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			assertV3ErrorResponse(
				t,
				recorder,
				tc.expectedStatus,
				tc.expectedCode,
				recorder.Header().Get(requestIDHeader),
			)
		})
	}
}
