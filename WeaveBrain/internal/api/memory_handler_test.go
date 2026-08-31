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

type fakeMemoryUseCase struct {
	listFn       func(context.Context, uuid.UUID, entity.MemoryListQuery) (*entity.MemoryListResult, error)
	detailFn     func(context.Context, uuid.UUID, uuid.UUID) (*entity.MemoryAggregate, error)
	correctFn    func(context.Context, uuid.UUID, uuid.UUID, service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error)
	appendNoteFn func(context.Context, uuid.UUID, uuid.UUID, string) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error)
	setPinnedFn  func(context.Context, uuid.UUID, uuid.UUID, bool) (*entity.CaptureAggregate, error)
	archiveFn    func(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAggregate, error)
	deleteFn     func(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAggregate, error)
}

func (f *fakeMemoryUseCase) List(ctx context.Context, userID uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error) {
	if f.listFn == nil {
		return &entity.MemoryListResult{}, nil
	}
	return f.listFn(ctx, userID, q)
}

func (f *fakeMemoryUseCase) GetDetail(ctx context.Context, userID, captureID uuid.UUID) (*entity.MemoryAggregate, error) {
	if f.detailFn == nil {
		return &entity.MemoryAggregate{}, nil
	}
	return f.detailFn(ctx, userID, captureID)
}

func (f *fakeMemoryUseCase) Correct(ctx context.Context, userID, captureID uuid.UUID, input service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
	if f.correctFn == nil {
		return &entity.EnrichmentRevision{}, &entity.CaptureAggregate{}, nil
	}
	return f.correctFn(ctx, userID, captureID, input)
}

func (f *fakeMemoryUseCase) AppendNote(ctx context.Context, userID, captureID uuid.UUID, text string) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
	if f.appendNoteFn == nil {
		return &entity.EnrichmentRevision{}, &entity.CaptureAggregate{}, nil
	}
	return f.appendNoteFn(ctx, userID, captureID, text)
}

func (f *fakeMemoryUseCase) SetPinned(ctx context.Context, userID, captureID uuid.UUID, pinned bool) (*entity.CaptureAggregate, error) {
	if f.setPinnedFn == nil {
		return &entity.CaptureAggregate{}, nil
	}
	return f.setPinnedFn(ctx, userID, captureID, pinned)
}

func (f *fakeMemoryUseCase) Archive(ctx context.Context, userID, captureID uuid.UUID) (*entity.CaptureAggregate, error) {
	if f.archiveFn == nil {
		return &entity.CaptureAggregate{}, nil
	}
	return f.archiveFn(ctx, userID, captureID)
}

func (f *fakeMemoryUseCase) Delete(ctx context.Context, userID, captureID uuid.UUID) (*entity.CaptureAggregate, error) {
	if f.deleteFn == nil {
		return &entity.CaptureAggregate{}, nil
	}
	return f.deleteFn(ctx, userID, captureID)
}

func newMemoryTestEngine(userID uuid.UUID, useCase memoryUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		SetCurrentUserID(c, userID)
		c.Next()
	})
	NewMemoryHandler(useCase).RegisterRoutes(group)
	return engine
}

func memoryTestEntry(userID uuid.UUID) *entity.MemoryListEntry {
	capture := &entity.Capture{
		ID: uuid.New(), UserID: userID, Kind: entity.CaptureKindText,
		OriginalText: strPtr2("原始记忆"), LifecycleStatus: "active",
	}
	card := &entity.MemoryCard{
		ID: uuid.New(), UserID: userID, CaptureID: capture.ID,
		PrimaryType: "idea", Title: "记忆标题", Tags: []string{"a"}, KeyPoints: []string{},
		ProcessingStatus: "ready",
	}
	return &entity.MemoryListEntry{Capture: capture, MemoryCard: card}
}

func strPtr2(s string) *string { return &s }

func defaultMemoryUseCase(userID uuid.UUID) *fakeMemoryUseCase {
	entry := memoryTestEntry(userID)
	revision := &entity.EnrichmentRevision{
		ID: 1, UserID: userID, CaptureID: entry.Capture.ID, Revision: 1,
		CardVersion: 1, Source: entity.EnrichmentSourceFallback, SourceRevision: 1,
	}
	aggregate := &entity.CaptureAggregate{Capture: entry.Capture, MemoryCard: entry.MemoryCard}
	return &fakeMemoryUseCase{
		listFn: func(_ context.Context, u uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error) {
			if u != userID {
				return nil, service.ErrInvalidMemoryInput
			}
			return &entity.MemoryListResult{Items: []*entity.MemoryListEntry{entry}}, nil
		},
		detailFn: func(_ context.Context, u uuid.UUID, captureID uuid.UUID) (*entity.MemoryAggregate, error) {
			if u != userID {
				return nil, service.ErrInvalidMemoryInput
			}
			return &entity.MemoryAggregate{
				Capture: entry.Capture, MemoryCard: entry.MemoryCard,
				Revisions: []*entity.EnrichmentRevision{revision},
			}, nil
		},
		correctFn: func(_ context.Context, u uuid.UUID, captureID uuid.UUID, input service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
			return revision, aggregate, nil
		},
		appendNoteFn: func(_ context.Context, u uuid.UUID, captureID uuid.UUID, text string) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
			return revision, aggregate, nil
		},
		setPinnedFn: func(_ context.Context, u uuid.UUID, captureID uuid.UUID, pinned bool) (*entity.CaptureAggregate, error) {
			return aggregate, nil
		},
		archiveFn: func(_ context.Context, u uuid.UUID, captureID uuid.UUID) (*entity.CaptureAggregate, error) {
			return aggregate, nil
		},
		deleteFn: func(_ context.Context, u uuid.UUID, captureID uuid.UUID) (*entity.CaptureAggregate, error) {
			return aggregate, nil
		},
	}
}

func TestMemoryHandlerListReturns200WithNextCursor(t *testing.T) {
	userID := uuid.New()
	entry := memoryTestEntry(userID)
	next := "next-page"
	useCase := defaultMemoryUseCase(userID)
	useCase.listFn = func(context.Context, uuid.UUID, entity.MemoryListQuery) (*entity.MemoryListResult, error) {
		return &entity.MemoryListResult{
			Items:      []*entity.MemoryListEntry{entry},
			NextCursor: &next,
		}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v3/memories", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp MemoryListResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.NextCursor == nil || *resp.NextCursor != "next-page" {
		t.Fatalf("expected next_cursor, got %#v", resp.NextCursor)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestMemoryHandlerListPassesFilters(t *testing.T) {
	userID := uuid.New()
	var gotQ entity.MemoryListQuery
	useCase := defaultMemoryUseCase(userID)
	useCase.listFn = func(_ context.Context, u uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error) {
		gotQ = q
		return &entity.MemoryListResult{}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v3/memories?limit=5&q=%E4%BD%A0%E5%A5%BD&kind=text&primary_type=idea&lifecycle_status=archived&pinned=true&cursor=abc", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotQ.Limit != 5 {
		t.Fatalf("expected limit 5, got %d", gotQ.Limit)
	}
	if gotQ.Q != "你好" {
		t.Fatalf("expected q 你好, got %q", gotQ.Q)
	}
	if gotQ.Kind != "text" || gotQ.PrimaryType != "idea" || gotQ.LifecycleStatus != "archived" {
		t.Fatalf("unexpected filters: %#v", gotQ)
	}
	if !gotQ.PinnedOnly {
		t.Fatal("expected pinned_only true")
	}
	if gotQ.Cursor == nil || *gotQ.Cursor != "abc" {
		t.Fatalf("unexpected cursor %#v", gotQ.Cursor)
	}
}

func TestMemoryHandlerListInvalidLimit(t *testing.T) {
	userID := uuid.New()
	engine := newMemoryTestEngine(userID, defaultMemoryUseCase(userID))

	for _, raw := range []string{"abc", "0", "51", "-1"} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v3/memories?limit="+raw, nil))
		assertV3ErrorResponse(
			t,
			recorder,
			http.StatusBadRequest,
			V3ErrorInvalidArgument,
			recorder.Header().Get(requestIDHeader),
		)
	}
}

func TestMemoryHandlerListInvalidPinned(t *testing.T) {
	userID := uuid.New()
	engine := newMemoryTestEngine(userID, defaultMemoryUseCase(userID))

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v3/memories?pinned=maybe", nil))
	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusBadRequest,
		V3ErrorInvalidArgument,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestMemoryHandlerListRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) { c.Next() }) // no user
	NewMemoryHandler(defaultMemoryUseCase(uuid.New())).RegisterRoutes(group)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v3/memories", nil))
	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusUnauthorized,
		V3ErrorUnauthorized,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestMemoryHandlerGetDetailReturns200(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	useCase := defaultMemoryUseCase(userID)
	useCase.detailFn = func(_ context.Context, u uuid.UUID, got uuid.UUID) (*entity.MemoryAggregate, error) {
		if got != captureID {
			t.Fatalf("detail called with %v, want %v", got, captureID)
		}
		return &entity.MemoryAggregate{
			Capture: &entity.Capture{ID: captureID, UserID: userID, LifecycleStatus: "active"},
			MemoryCard: &entity.MemoryCard{ID: uuid.New(), CaptureID: captureID, Title: "标题"},
			Revisions: []*entity.EnrichmentRevision{
				{ID: 1, Revision: 1, Source: entity.EnrichmentSourceFallback},
			},
		}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v3/memories/"+captureID.String(), nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp MemoryDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Capture == nil || resp.Capture.ID != captureID {
		t.Fatalf("unexpected capture in detail: %#v", resp.Capture)
	}
	if resp.MemoryCard == nil || resp.MemoryCard.Title != "标题" {
		t.Fatalf("unexpected memory card in detail: %#v", resp.MemoryCard)
	}
	if len(resp.Revisions) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(resp.Revisions))
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestMemoryHandlerGetDetailNotFound(t *testing.T) {
	userID := uuid.New()
	useCase := defaultMemoryUseCase(userID)
	useCase.detailFn = func(context.Context, uuid.UUID, uuid.UUID) (*entity.MemoryAggregate, error) {
		return nil, service.ErrMemoryNotFound
	}
	engine := newMemoryTestEngine(userID, useCase)

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v3/memories/"+uuid.New().String(), nil))
	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusNotFound,
		V3ErrorNotFound,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestMemoryHandlerGetDetailInvalidID(t *testing.T) {
	userID := uuid.New()
	engine := newMemoryTestEngine(userID, defaultMemoryUseCase(userID))

	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v3/memories/not-a-uuid", nil))
	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusBadRequest,
		V3ErrorInvalidArgument,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestMemoryHandlerCorrectReturns200(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	var gotInput service.CorrectMemoryInput
	useCase := defaultMemoryUseCase(userID)
	useCase.correctFn = func(_ context.Context, u uuid.UUID, got uuid.UUID, input service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
		if got != captureID {
			t.Fatalf("correct called with %v, want %v", got, captureID)
		}
		gotInput = input
		return &entity.EnrichmentRevision{ID: 2, Revision: 2, Source: entity.EnrichmentSourceUser},
			&entity.CaptureAggregate{Capture: &entity.Capture{ID: captureID}, MemoryCard: &entity.MemoryCard{Title: "新标题"}}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	body := `{"title":"新标题","summary":"摘要","primary_type":"idea","tags":["a","b"],"key_points":["kp"]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/memories/"+captureID.String(), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotInput.Title == nil || *gotInput.Title != "新标题" {
		t.Fatalf("unexpected title input: %#v", gotInput.Title)
	}
	if gotInput.Summary == nil || *gotInput.Summary != "摘要" {
		t.Fatalf("unexpected summary input: %#v", gotInput.Summary)
	}
	if gotInput.PrimaryType == nil || *gotInput.PrimaryType != "idea" {
		t.Fatalf("unexpected primary_type input: %#v", gotInput.PrimaryType)
	}
	if len(gotInput.Tags) != 2 || len(gotInput.KeyPoints) != 1 {
		t.Fatalf("unexpected tags/key_points input: %#v/%#v", gotInput.Tags, gotInput.KeyPoints)
	}

	var resp MemoryMutationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Revision == nil || resp.Revision.Source != entity.EnrichmentSourceUser {
		t.Fatalf("expected user revision in response, got %#v", resp.Revision)
	}
	if resp.MemoryCard == nil || resp.MemoryCard.Title != "新标题" {
		t.Fatalf("expected updated card in response, got %#v", resp.MemoryCard)
	}
	if resp.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestMemoryHandlerCorrectOnlySendsProvidedFields(t *testing.T) {
	userID := uuid.New()
	var gotInput service.CorrectMemoryInput
	useCase := defaultMemoryUseCase(userID)
	useCase.correctFn = func(_ context.Context, u uuid.UUID, got uuid.UUID, input service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
		gotInput = input
		return &entity.EnrichmentRevision{}, &entity.CaptureAggregate{}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPatch, "/api/v3/memories/"+uuid.New().String(), strings.NewReader(`{"title":"仅标题"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotInput.Title == nil || *gotInput.Title != "仅标题" {
		t.Fatalf("expected title only, got %#v", gotInput)
	}
	if gotInput.Summary != nil || gotInput.PrimaryType != nil || gotInput.Tags != nil || gotInput.KeyPoints != nil {
		t.Fatalf("absent fields must stay nil, got %#v", gotInput)
	}
}

func TestMemoryHandlerCorrectInvalidBody(t *testing.T) {
	userID := uuid.New()
	engine := newMemoryTestEngine(userID, defaultMemoryUseCase(userID))

	for _, body := range []string{"{", `{"title":123}`} {
		req := httptest.NewRequest(http.MethodPatch, "/api/v3/memories/"+uuid.New().String(), strings.NewReader(body))
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

func TestMemoryHandlerCorrectMapsInvalidInput(t *testing.T) {
	userID := uuid.New()
	useCase := defaultMemoryUseCase(userID)
	useCase.correctFn = func(context.Context, uuid.UUID, uuid.UUID, service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
		return nil, nil, service.ErrInvalidMemoryInput
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPatch, "/api/v3/memories/"+uuid.New().String(), strings.NewReader(`{"title":"x"}`))
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

func TestMemoryHandlerCorrectVersionConflict(t *testing.T) {
	userID := uuid.New()
	useCase := defaultMemoryUseCase(userID)
	useCase.correctFn = func(context.Context, uuid.UUID, uuid.UUID, service.CorrectMemoryInput) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
		return nil, nil, service.ErrMemoryVersionConflict
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPatch, "/api/v3/memories/"+uuid.New().String(), strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusConflict,
		V3ErrorVersionConflict,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestMemoryHandlerAppendNoteReturns200(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	var gotText string
	useCase := defaultMemoryUseCase(userID)
	useCase.appendNoteFn = func(_ context.Context, u uuid.UUID, got uuid.UUID, text string) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
		if got != captureID {
			t.Fatalf("append note called with %v, want %v", got, captureID)
		}
		gotText = text
		return &entity.EnrichmentRevision{ID: 3, Revision: 2}, &entity.CaptureAggregate{}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/memories/"+captureID.String()+"/notes", strings.NewReader(`{"text":"继续思考"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if gotText != "继续思考" {
		t.Fatalf("unexpected note text %q", gotText)
	}
	var resp MemoryMutationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Revision == nil || resp.RequestID == "" {
		t.Fatalf("expected revision and request_id in response, got %#v", resp)
	}
}

func TestMemoryHandlerAppendNoteEmptyText(t *testing.T) {
	userID := uuid.New()
	useCase := defaultMemoryUseCase(userID)
	useCase.appendNoteFn = func(context.Context, uuid.UUID, uuid.UUID, string) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
		return nil, nil, service.ErrInvalidMemoryInput
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/memories/"+uuid.New().String()+"/notes", strings.NewReader(`{"text":""}`))
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

func TestMemoryHandlerSetPinned(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	var gotPinned bool
	useCase := defaultMemoryUseCase(userID)
	useCase.setPinnedFn = func(_ context.Context, u uuid.UUID, got uuid.UUID, pinned bool) (*entity.CaptureAggregate, error) {
		gotPinned = pinned
		return &entity.CaptureAggregate{Capture: &entity.Capture{ID: captureID}, MemoryCard: &entity.MemoryCard{IsPinned: pinned}}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/memories/"+captureID.String()+"/pin", strings.NewReader(`{"pinned":true}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !gotPinned {
		t.Fatal("expected pinned=true passed through")
	}
	var resp MemoryMutationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.MemoryCard == nil || !resp.MemoryCard.IsPinned || resp.RequestID == "" {
		t.Fatalf("expected pinned card and request_id, got %#v", resp)
	}
}

func TestMemoryHandlerArchive(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	var called bool
	useCase := defaultMemoryUseCase(userID)
	useCase.archiveFn = func(_ context.Context, u uuid.UUID, got uuid.UUID) (*entity.CaptureAggregate, error) {
		if got != captureID {
			t.Fatalf("archive called with %v, want %v", got, captureID)
		}
		called = true
		return &entity.CaptureAggregate{Capture: &entity.Capture{ID: captureID, LifecycleStatus: "archived"}}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/memories/"+captureID.String()+"/archive", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !called {
		t.Fatal("expected archive called")
	}
	var resp MemoryMutationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Capture == nil || resp.Capture.LifecycleStatus != "archived" || resp.RequestID == "" {
		t.Fatalf("expected archived capture and request_id, got %#v", resp)
	}
}

func TestMemoryHandlerDelete(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	var called bool
	useCase := defaultMemoryUseCase(userID)
	useCase.deleteFn = func(_ context.Context, u uuid.UUID, got uuid.UUID) (*entity.CaptureAggregate, error) {
		if got != captureID {
			t.Fatalf("delete called with %v, want %v", got, captureID)
		}
		called = true
		return &entity.CaptureAggregate{Capture: &entity.Capture{ID: captureID, LifecycleStatus: "trashed"}}, nil
	}
	engine := newMemoryTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodDelete, "/api/v3/memories/"+captureID.String(), nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !called {
		t.Fatal("expected delete called")
	}
	var resp MemoryMutationResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Capture == nil || resp.Capture.LifecycleStatus != "trashed" || resp.RequestID == "" {
		t.Fatalf("expected trashed capture and request_id, got %#v", resp)
	}
}

func TestMemoryHandlerMutationsRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) { c.Next() }) // no user
	NewMemoryHandler(defaultMemoryUseCase(uuid.New())).RegisterRoutes(group)

	captureID := uuid.New().String()
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPatch, "/api/v3/memories/" + captureID, `{"title":"x"}`},
		{http.MethodPost, "/api/v3/memories/" + captureID + "/notes", `{"text":"x"}`},
		{http.MethodPost, "/api/v3/memories/" + captureID + "/pin", `{"pinned":true}`},
		{http.MethodPost, "/api/v3/memories/" + captureID + "/archive", ``},
		{http.MethodDelete, "/api/v3/memories/" + captureID, ``},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
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
