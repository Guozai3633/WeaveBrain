package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeCompletionUseCase struct {
	previewFn func(context.Context, uuid.UUID, uuid.UUID) (*entity.CompletionPreviewResult, error)
	applyFn   func(context.Context, uuid.UUID, uuid.UUID, service.ApplyCompletionInput) (*entity.CompletionApplyResult, error)
	undoFn    func(context.Context, uuid.UUID, uuid.UUID) (*entity.CompletionApplyResult, error)
}

func (f *fakeCompletionUseCase) Preview(ctx context.Context, userID, captureID uuid.UUID) (*entity.CompletionPreviewResult, error) {
	if f.previewFn == nil {
		return &entity.CompletionPreviewResult{CaptureID: captureID}, nil
	}
	return f.previewFn(ctx, userID, captureID)
}

func (f *fakeCompletionUseCase) Apply(ctx context.Context, userID, captureID uuid.UUID, input service.ApplyCompletionInput) (*entity.CompletionApplyResult, error) {
	if f.applyFn == nil {
		return &entity.CompletionApplyResult{Card: &entity.MemoryCard{CaptureID: captureID}}, nil
	}
	return f.applyFn(ctx, userID, captureID, input)
}

func (f *fakeCompletionUseCase) Undo(ctx context.Context, userID, captureID uuid.UUID) (*entity.CompletionApplyResult, error) {
	if f.undoFn == nil {
		return &entity.CompletionApplyResult{Card: &entity.MemoryCard{CaptureID: captureID}}, nil
	}
	return f.undoFn(ctx, userID, captureID)
}

func newCompletionTestEngine(userID uuid.UUID, useCase completionUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		SetCurrentUserID(c, userID)
		c.Next()
	})
	NewCompletionHandler(useCase).RegisterRoutes(group)
	return engine
}

func completionTestProposal(field string) *entity.CompletionProposal {
	return &entity.CompletionProposal{
		ID:            uuid.New(),
		CaptureID:     uuid.New(),
		PreviewID:     uuid.New(),
		SourceRevision: 1,
		FieldName:     field,
		OriginalValue: "",
		ProposedValue: "建议值",
		Provenance:    entity.ProvenanceAI,
		ApplyPolicy:   entity.ApplyPolicySafeAuto,
		Status:        entity.ProposalStatusPending,
		EvidenceSpans: []entity.EvidenceSpan{{Start: 0, End: 3, Quote: "原文"}},
		Provider:      "ollama",
		Model:         "qwen2.5:7b",
		ConfigVersion: "completion-prompt-v1",
	}
}

func TestCompletionHandlerPreviewReturns200(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	proposal := completionTestProposal("tags")
	useCase := &fakeCompletionUseCase{
		previewFn: func(_ context.Context, u uuid.UUID, got uuid.UUID) (*entity.CompletionPreviewResult, error) {
			if got != captureID {
				t.Fatalf("preview called with %v, want %v", got, captureID)
			}
			return &entity.CompletionPreviewResult{
				CaptureID:      captureID,
				SourceRevision: 3,
				MissingFields:  []string{"tags", "key_points"},
				Proposals:      []*entity.CompletionProposal{proposal},
			}, nil
		},
	}
	engine := newCompletionTestEngine(userID, useCase)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v3/captures/"+captureID.String()+"/completion/preview", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp CompletionPreviewResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Preview == nil || resp.Preview.CaptureID != captureID {
		t.Fatalf("unexpected preview result: %#v", resp.Preview)
	}
	if resp.Preview.SourceRevision != 3 {
		t.Fatalf("expected source_revision 3, got %d", resp.Preview.SourceRevision)
	}
	if len(resp.Preview.MissingFields) != 2 {
		t.Fatalf("expected 2 missing fields, got %#v", resp.Preview.MissingFields)
	}
	if len(resp.Preview.Proposals) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(resp.Preview.Proposals))
	}
	if resp.Preview.Proposals[0].FieldName != "tags" || resp.Preview.Proposals[0].ApplyPolicy != entity.ApplyPolicySafeAuto {
		t.Fatalf("unexpected proposal: %#v", resp.Preview.Proposals[0])
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestCompletionHandlerApplyReturns200(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	proposalID := uuid.New()
	card := &entity.MemoryCard{ID: uuid.New(), CaptureID: captureID, Title: "补全标题", ProcessingStatus: "ready"}
	var gotInput service.ApplyCompletionInput
	useCase := &fakeCompletionUseCase{
		applyFn: func(_ context.Context, u uuid.UUID, got uuid.UUID, input service.ApplyCompletionInput) (*entity.CompletionApplyResult, error) {
			if got != captureID {
				t.Fatalf("apply called with %v, want %v", got, captureID)
			}
			gotInput = input
			return &entity.CompletionApplyResult{
				Card:               card,
				AppliedProposalIDs: []uuid.UUID{proposalID},
			}, nil
		},
	}
	engine := newCompletionTestEngine(userID, useCase)

	body := `{"proposal_ids":["` + proposalID.String() + `"],"source_revision":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v3/captures/"+captureID.String()+"/completion/apply", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if len(gotInput.ProposalIDs) != 1 || gotInput.ProposalIDs[0] != proposalID {
		t.Fatalf("unexpected proposal_ids input: %#v", gotInput.ProposalIDs)
	}
	if gotInput.SourceRevision != 3 {
		t.Fatalf("expected source_revision 3, got %d", gotInput.SourceRevision)
	}
	var resp CompletionApplyResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Apply == nil || resp.Apply.Card == nil || resp.Apply.Card.Title != "补全标题" {
		t.Fatalf("unexpected apply result: %#v", resp.Apply)
	}
	if len(resp.Apply.AppliedProposalIDs) != 1 || resp.Apply.AppliedProposalIDs[0] != proposalID {
		t.Fatalf("unexpected applied_proposal_ids: %#v", resp.Apply.AppliedProposalIDs)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestCompletionHandlerApplyInvalidBody(t *testing.T) {
	userID := uuid.New()
	engine := newCompletionTestEngine(userID, &fakeCompletionUseCase{})

	for _, body := range []string{"{", `{"proposal_ids":"not-an-array"}`} {
		req := httptest.NewRequest(http.MethodPost, "/api/v3/captures/"+uuid.New().String()+"/completion/apply", strings.NewReader(body))
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

func TestCompletionHandlerUndoReturns200(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	card := &entity.MemoryCard{ID: uuid.New(), CaptureID: captureID, Title: "回滚标题", ProcessingStatus: "ready"}
	useCase := &fakeCompletionUseCase{
		undoFn: func(_ context.Context, u uuid.UUID, got uuid.UUID) (*entity.CompletionApplyResult, error) {
			if got != captureID {
				t.Fatalf("undo called with %v, want %v", got, captureID)
			}
			return &entity.CompletionApplyResult{Card: card}, nil
		},
	}
	engine := newCompletionTestEngine(userID, useCase)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v3/captures/"+captureID.String()+"/completion/undo", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp CompletionUndoResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Undo == nil || resp.Undo.Card == nil || resp.Undo.Card.Title != "回滚标题" {
		t.Fatalf("unexpected undo result: %#v", resp.Undo)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestCompletionHandlerRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) { c.Next() }) // no user
	NewCompletionHandler(&fakeCompletionUseCase{}).RegisterRoutes(group)

	captureID := uuid.New().String()
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v3/captures/" + captureID + "/completion/preview", ``},
		{http.MethodPost, "/api/v3/captures/" + captureID + "/completion/apply", `{"proposal_ids":[],"source_revision":1}`},
		{http.MethodPost, "/api/v3/captures/" + captureID + "/completion/undo", ``},
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

func TestCompletionHandlerInvalidCaptureID(t *testing.T) {
	userID := uuid.New()
	engine := newCompletionTestEngine(userID, &fakeCompletionUseCase{})

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v3/captures/not-a-uuid/completion/preview"},
		{http.MethodPost, "/api/v3/captures/not-a-uuid/completion/apply"},
		{http.MethodPost, "/api/v3/captures/not-a-uuid/completion/undo"},
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, nil))
		assertV3ErrorResponse(
			t,
			recorder,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			recorder.Header().Get(requestIDHeader),
		)
	}
}

func TestCompletionHandlerErrorMappings(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()

	previewErr := func(err error) *fakeCompletionUseCase {
		return &fakeCompletionUseCase{
			previewFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.CompletionPreviewResult, error) {
				return nil, err
			},
		}
	}
	applyErr := func(err error) *fakeCompletionUseCase {
		return &fakeCompletionUseCase{
			applyFn: func(context.Context, uuid.UUID, uuid.UUID, service.ApplyCompletionInput) (*entity.CompletionApplyResult, error) {
				return nil, err
			},
		}
	}
	undoErr := func(err error) *fakeCompletionUseCase {
		return &fakeCompletionUseCase{
			undoFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.CompletionApplyResult, error) {
				return nil, err
			},
		}
	}

	cases := []struct {
		name           string
		useCase        *fakeCompletionUseCase
		path           string
		body           string
		expectedStatus int
		expectedCode   V3ErrorCode
	}{
		{"preview disabled", previewErr(service.ErrCompletionDisabled), "/completion/preview", "", http.StatusConflict, V3ErrorFeatureNotEnabled},
		{"preview not ready", previewErr(service.ErrCompletionNotReady), "/completion/preview", "", http.StatusConflict, V3ErrorPreconditionFailed},
		{"preview not found", previewErr(service.ErrMemoryNotFound), "/completion/preview", "", http.StatusNotFound, V3ErrorNotFound},
		{"preview llm error", previewErr(service.ErrCompletionLLM), "/completion/preview", "", http.StatusInternalServerError, V3ErrorInternal},
		{"apply disabled", applyErr(service.ErrCompletionDisabled), "/completion/apply", `{"proposal_ids":["` + uuid.NewString() + `"],"source_revision":1}`, http.StatusConflict, V3ErrorFeatureNotEnabled},
		{"apply version conflict", applyErr(service.ErrCompletionVersionConflict), "/completion/apply", `{"proposal_ids":["` + uuid.NewString() + `"],"source_revision":1}`, http.StatusConflict, V3ErrorVersionConflict},
		{"apply invalid input", applyErr(service.ErrInvalidCompletionInput), "/completion/apply", `{"proposal_ids":["` + uuid.NewString() + `"],"source_revision":1}`, http.StatusBadRequest, V3ErrorInvalidArgument},
		{"apply not ready", applyErr(service.ErrCompletionNotReady), "/completion/apply", `{"proposal_ids":["` + uuid.NewString() + `"],"source_revision":1}`, http.StatusConflict, V3ErrorPreconditionFailed},
		{"apply not found", applyErr(service.ErrMemoryNotFound), "/completion/apply", `{"proposal_ids":["` + uuid.NewString() + `"],"source_revision":1}`, http.StatusNotFound, V3ErrorNotFound},
		{"undo nothing", undoErr(service.ErrNothingToUndo), "/completion/undo", "", http.StatusConflict, V3ErrorPreconditionFailed},
		{"undo not found", undoErr(service.ErrMemoryNotFound), "/completion/undo", "", http.StatusNotFound, V3ErrorNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			engine := newCompletionTestEngine(userID, tc.useCase)
			req := httptest.NewRequest(http.MethodPost, "/api/v3/captures/"+captureID.String()+tc.path, strings.NewReader(tc.body))
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
