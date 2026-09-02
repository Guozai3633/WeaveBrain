package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeEchoUseCase struct {
	currentFn         func(context.Context, uuid.UUID) (*service.EchoCurrentResult, error)
	feedbackFn        func(context.Context, uuid.UUID, uuid.UUID, entity.EchoFeedbackVerdict) (*service.EchoFeedbackResult, error)
	getSettingsFn     func(context.Context, uuid.UUID) (*entity.UserEchoSettings, error)
	updateSettingsFn  func(context.Context, service.UpdateEchoSettingsInput) (*entity.UserEchoSettings, error)
}

func (f *fakeEchoUseCase) CurrentEcho(ctx context.Context, userID uuid.UUID) (*service.EchoCurrentResult, error) {
	return f.currentFn(ctx, userID)
}

func (f *fakeEchoUseCase) Feedback(ctx context.Context, userID uuid.UUID, echoID uuid.UUID, verdict entity.EchoFeedbackVerdict) (*service.EchoFeedbackResult, error) {
	return f.feedbackFn(ctx, userID, echoID, verdict)
}

func (f *fakeEchoUseCase) GetSettings(ctx context.Context, userID uuid.UUID) (*entity.UserEchoSettings, error) {
	return f.getSettingsFn(ctx, userID)
}

func (f *fakeEchoUseCase) UpdateSettings(ctx context.Context, input service.UpdateEchoSettingsInput) (*entity.UserEchoSettings, error) {
	return f.updateSettingsFn(ctx, input)
}

func newEchoTestEngine(userID uuid.UUID, useCase echoUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		SetCurrentUserID(c, userID)
		c.Next()
	})
	NewEchoHandler(useCase).RegisterRoutes(group)
	return engine
}

func echoUseCaseDefaults(userID uuid.UUID) *fakeEchoUseCase {
	settings := &entity.UserEchoSettings{
		UserID: userID, Enabled: false, Cadence: entity.EchoCadenceDaily,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	return &fakeEchoUseCase{
		currentFn: func(_ context.Context, got uuid.UUID) (*service.EchoCurrentResult, error) {
			if got != userID {
				return nil, repository.ErrEchoNotFound
			}
			return &service.EchoCurrentResult{Enabled: false, Cadence: entity.EchoCadenceDaily, Revision: 0}, nil
		},
		feedbackFn: func(context.Context, uuid.UUID, uuid.UUID, entity.EchoFeedbackVerdict) (*service.EchoFeedbackResult, error) {
			return nil, repository.ErrEchoNotFound
		},
		getSettingsFn: func(context.Context, uuid.UUID) (*entity.UserEchoSettings, error) {
			return settings, nil
		},
		updateSettingsFn: func(_ context.Context, input service.UpdateEchoSettingsInput) (*entity.UserEchoSettings, error) {
			updated := *settings
			updated.Revision = input.ExpectedRevision + 1
			if input.Enabled != nil {
				updated.Enabled = *input.Enabled
			}
			if input.Cadence != nil {
				updated.Cadence = *input.Cadence
			}
			return &updated, nil
		},
	}
}

func loadedEchoResult(userID uuid.UUID) *service.EchoCurrentResult {
	summary := "一条示例记忆"
	return &service.EchoCurrentResult{
		Enabled:  true,
		Cadence:  entity.EchoCadenceDaily,
		Revision: 2,
		Echo: &service.EchoCurrentCard{
			EchoID:    uuid.New(),
			Status:    entity.EchoStatusOpen,
			Reason:    entity.EchoReason{Code: entity.EchoReasonPinned, Text: "这条记忆被你置顶过，适合专门回看"},
			CreatedAt: time.Now().UTC(),
			Memory: entity.EchoMemory{
				CaptureID:   uuid.New(),
				Kind:        "text",
				Title:       "织脑的想法",
				Summary:     &summary,
				PrimaryType: "idea",
				IsPinned:    true,
			},
		},
	}
}

func TestEchoHandlerCurrentReturnsCard(t *testing.T) {
	userID := uuid.New()
	result := loadedEchoResult(userID)
	useCase := echoUseCaseDefaults(userID)
	useCase.currentFn = func(context.Context, uuid.UUID) (*service.EchoCurrentResult, error) { return result, nil }
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/echoes/current", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response EchoCurrentResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !response.Enabled || response.Revision != 2 {
		t.Fatalf("unexpected envelope: %#v", response)
	}
	if response.Echo == nil {
		t.Fatal("expected echo card")
	}
	if response.Echo.ID != result.Echo.EchoID ||
		response.Echo.Reason.Code != entity.EchoReasonPinned ||
		response.Echo.Reason.Text == "" ||
		response.Echo.Memory.CaptureID != result.Echo.Memory.CaptureID ||
		!response.Echo.Memory.IsPinned {
		t.Fatalf("unexpected card payload: %#v", response.Echo)
	}
	if response.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestEchoHandlerCurrentDisabledReturnsEmpty(t *testing.T) {
	userID := uuid.New()
	engine := newEchoTestEngine(userID, echoUseCaseDefaults(userID))

	req := httptest.NewRequest(http.MethodGet, "/api/v3/echoes/current", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response EchoCurrentResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Enabled {
		t.Fatalf("expected disabled, got %#v", response)
	}
	if response.Echo != nil {
		t.Fatalf("disabled must not carry an echo card")
	}
	if response.Cadence != entity.EchoCadenceDaily {
		t.Fatalf("cadence must still be present, got %#v", response)
	}
}

func TestEchoHandlerCurrentBetweenWindowsReturnsNextDue(t *testing.T) {
	userID := uuid.New()
	nextDue := time.Now().UTC().Add(24 * time.Hour)
	useCase := echoUseCaseDefaults(userID)
	useCase.currentFn = func(context.Context, uuid.UUID) (*service.EchoCurrentResult, error) {
		return &service.EchoCurrentResult{
			Enabled: true, Cadence: entity.EchoCadenceDaily, Revision: 1, NextDueAt: &nextDue,
		}, nil
	}
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/echoes/current", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	var response EchoCurrentResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Echo != nil {
		t.Fatalf("between windows must not carry an echo card")
	}
	if response.NextDueAt == nil {
		t.Fatal("expected next_due_at")
	}
}

func TestEchoHandlerCurrentNoCandidatesReturnsEmptyReason(t *testing.T) {
	userID := uuid.New()
	useCase := echoUseCaseDefaults(userID)
	useCase.currentFn = func(context.Context, uuid.UUID) (*service.EchoCurrentResult, error) {
		return &service.EchoCurrentResult{
			Enabled: true, Cadence: entity.EchoCadenceDaily, Revision: 1, EmptyReason: "no_candidates",
		}, nil
	}
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/echoes/current", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	var response EchoCurrentResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Echo != nil || response.EmptyReason != "no_candidates" {
		t.Fatalf("expected no_candidates, got %#v", response)
	}
}

func TestEchoHandlerCurrentRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) { c.Next() })
	NewEchoHandler(echoUseCaseDefaults(uuid.New())).RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/echoes/current", nil)
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

func TestEchoHandlerFeedbackPassesVerdict(t *testing.T) {
	userID := uuid.New()
	echoID := uuid.New()
	nextDue := time.Now().UTC().Add(24 * time.Hour)
	useCase := echoUseCaseDefaults(userID)
	useCase.feedbackFn = func(_ context.Context, got uuid.UUID, gotEcho uuid.UUID, verdict entity.EchoFeedbackVerdict) (*service.EchoFeedbackResult, error) {
		if got != userID || gotEcho != echoID || verdict != entity.EchoVerdictDone {
			t.Fatalf("unexpected feedback args: user=%v echo=%v verdict=%v", got, gotEcho, verdict)
		}
		return &service.EchoFeedbackResult{EchoID: echoID, Status: entity.EchoStatusDone, NextDueAt: nextDue}, nil
	}
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/echoes/"+echoID.String()+"/feedback", strings.NewReader(`{"verdict":"done"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response EchoFeedbackResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.EchoID != echoID || response.Status != entity.EchoStatusDone {
		t.Fatalf("unexpected feedback response: %#v", response)
	}
	if response.NextDueAt.IsZero() {
		t.Fatal("expected next_due_at")
	}
}

func TestEchoHandlerFeedbackMissingVerdictIsBadRequest(t *testing.T) {
	userID := uuid.New()
	engine := newEchoTestEngine(userID, echoUseCaseDefaults(userID))

	req := httptest.NewRequest(http.MethodPost, "/api/v3/echoes/"+uuid.New().String()+"/feedback", strings.NewReader(`{}`))
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

func TestEchoHandlerFeedbackNotOpenIsVersionConflict(t *testing.T) {
	userID := uuid.New()
	useCase := echoUseCaseDefaults(userID)
	useCase.feedbackFn = func(context.Context, uuid.UUID, uuid.UUID, entity.EchoFeedbackVerdict) (*service.EchoFeedbackResult, error) {
		return nil, repository.ErrEchoNotOpen
	}
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/echoes/"+uuid.New().String()+"/feedback", strings.NewReader(`{"verdict":"later"}`))
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

func TestEchoHandlerFeedbackUnknownEchoIsNotFound(t *testing.T) {
	userID := uuid.New()
	useCase := echoUseCaseDefaults(userID)
	useCase.feedbackFn = func(context.Context, uuid.UUID, uuid.UUID, entity.EchoFeedbackVerdict) (*service.EchoFeedbackResult, error) {
		return nil, repository.ErrEchoNotFound
	}
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/echoes/"+uuid.New().String()+"/feedback", strings.NewReader(`{"verdict":"done"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusNotFound,
		V3ErrorNotFound,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestEchoHandlerFeedbackRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) { c.Next() })
	NewEchoHandler(echoUseCaseDefaults(uuid.New())).RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/echoes/"+uuid.New().String()+"/feedback", strings.NewReader(`{"verdict":"done"}`))
	req.Header.Set("Content-Type", "application/json")
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

func TestEchoHandlerGetSettingsReturnsDefaults(t *testing.T) {
	userID := uuid.New()
	engine := newEchoTestEngine(userID, echoUseCaseDefaults(userID))

	req := httptest.NewRequest(http.MethodGet, "/api/v3/users/me/echo-settings", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response EchoSettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Settings == nil || response.Settings.UserID != userID {
		t.Fatalf("unexpected settings: %#v", response.Settings)
	}
	if response.Settings.Enabled {
		t.Fatal("echo settings must default to disabled")
	}
}

func TestEchoHandlerPatchSettingsPartialUpdate(t *testing.T) {
	userID := uuid.New()
	useCase := echoUseCaseDefaults(userID)
	var captured service.UpdateEchoSettingsInput
	useCase.updateSettingsFn = func(_ context.Context, input service.UpdateEchoSettingsInput) (*entity.UserEchoSettings, error) {
		captured = input
		updated := &entity.UserEchoSettings{
			UserID: userID, Enabled: false, Cadence: entity.EchoCadenceDaily,
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		updated.Revision = input.ExpectedRevision + 1
		if input.Enabled != nil {
			updated.Enabled = *input.Enabled
		}
		if input.Cadence != nil {
			updated.Cadence = *input.Cadence
		}
		return updated, nil
	}
	engine := newEchoTestEngine(userID, useCase)

	body := `{"expected_revision":0,"enabled":true,"cadence":"weekly"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/echo-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if captured.UserID != userID || captured.ExpectedRevision != 0 {
		t.Fatalf("unexpected captured input: %#v", captured)
	}
	if captured.Enabled == nil || !*captured.Enabled {
		t.Fatal("enabled must be passed as true")
	}
	if captured.Cadence == nil || *captured.Cadence != entity.EchoCadenceWeekly {
		t.Fatalf("cadence must be passed as weekly, got %#v", captured.Cadence)
	}
	var response EchoSettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Settings == nil || !response.Settings.Enabled ||
		response.Settings.Cadence != entity.EchoCadenceWeekly || response.Settings.Revision != 1 {
		t.Fatalf("unexpected updated settings: %#v", response.Settings)
	}
}

func TestEchoHandlerPatchSettingsMissingRevisionIsBadRequest(t *testing.T) {
	userID := uuid.New()
	engine := newEchoTestEngine(userID, echoUseCaseDefaults(userID))

	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/echo-settings", strings.NewReader(`{"enabled":true}`))
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

func TestEchoHandlerPatchSettingsNoFieldsIsBadRequest(t *testing.T) {
	userID := uuid.New()
	engine := newEchoTestEngine(userID, echoUseCaseDefaults(userID))

	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/echo-settings", strings.NewReader(`{"expected_revision":0}`))
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

func TestEchoHandlerPatchSettingsConflictMapsToVersionConflict(t *testing.T) {
	userID := uuid.New()
	useCase := echoUseCaseDefaults(userID)
	useCase.updateSettingsFn = func(context.Context, service.UpdateEchoSettingsInput) (*entity.UserEchoSettings, error) {
		return nil, service.ErrEchoSettingsConflict
	}
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/echo-settings", strings.NewReader(`{"expected_revision":0,"enabled":true}`))
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

func TestEchoHandlerPatchSettingsInvalidCadenceIsBadRequest(t *testing.T) {
	userID := uuid.New()
	useCase := echoUseCaseDefaults(userID)
	useCase.updateSettingsFn = func(context.Context, service.UpdateEchoSettingsInput) (*entity.UserEchoSettings, error) {
		return nil, service.ErrInvalidEchoSettings
	}
	engine := newEchoTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/echo-settings", strings.NewReader(`{"expected_revision":0,"cadence":"monthly"}`))
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

func TestEchoHandlerGetSettingsRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) { c.Next() })
	NewEchoHandler(echoUseCaseDefaults(uuid.New())).RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/users/me/echo-settings", nil)
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
