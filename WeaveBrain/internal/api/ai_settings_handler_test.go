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

type fakeAISettingsUseCase struct {
	getFn        func(context.Context, uuid.UUID) (*entity.UserAISettings, error)
	updateFn     func(context.Context, service.UpdateAISettingsInput) (*entity.UserAISettings, error)
	countFn      func(context.Context, uuid.UUID) (int64, error)
	reorganizeFn func(context.Context, uuid.UUID) (int64, error)
}

func (f *fakeAISettingsUseCase) Get(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error) {
	return f.getFn(ctx, userID)
}

func (f *fakeAISettingsUseCase) Update(ctx context.Context, input service.UpdateAISettingsInput) (*entity.UserAISettings, error) {
	return f.updateFn(ctx, input)
}

func (f *fakeAISettingsUseCase) CountPendingReorganize(ctx context.Context, userID uuid.UUID) (int64, error) {
	return f.countFn(ctx, userID)
}

func (f *fakeAISettingsUseCase) Reorganize(ctx context.Context, userID uuid.UUID) (int64, error) {
	return f.reorganizeFn(ctx, userID)
}

func newAISettingsTestEngine(userID uuid.UUID, useCase aiSettingsUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		SetCurrentUserID(c, userID)
		c.Next()
	})
	NewAISettingsHandler(useCase).RegisterRoutes(group)
	return engine
}

func defaultAISettingsUseCase(userID uuid.UUID) *fakeAISettingsUseCase {
	defaults := entity.DefaultUserAISettings(userID)
	return &fakeAISettingsUseCase{
		getFn: func(_ context.Context, got uuid.UUID) (*entity.UserAISettings, error) {
			if got != userID {
				return nil, service.ErrInvalidAISettings
			}
			return defaults, nil
		},
		updateFn: func(_ context.Context, input service.UpdateAISettingsInput) (*entity.UserAISettings, error) {
			if input.UserID != userID {
				return nil, service.ErrInvalidAISettings
			}
			updated := entity.DefaultUserAISettings(userID)
			updated.Revision = input.ExpectedRevision + 1
			if input.AIMemoryEnabled != nil {
				updated.AIMemoryEnabled = *input.AIMemoryEnabled
			}
			if input.AICompletionEnabled != nil {
				updated.AICompletionEnabled = *input.AICompletionEnabled
			}
			if input.SpeechToTextEnabled != nil {
				updated.SpeechToTextEnabled = *input.SpeechToTextEnabled
			}
			if input.CloudTextAllowed != nil {
				updated.CloudTextAllowed = *input.CloudTextAllowed
			}
			if input.CloudAudioAllowed != nil {
				updated.CloudAudioAllowed = *input.CloudAudioAllowed
			}
			return updated, nil
		},
		countFn: func(context.Context, uuid.UUID) (int64, error) {
			return 0, nil
		},
		reorganizeFn: func(context.Context, uuid.UUID) (int64, error) {
			return 0, nil
		},
	}
}

func TestAISettingsHandlerGetReturnsDefaults(t *testing.T) {
	userID := uuid.New()
	engine := newAISettingsTestEngine(userID, defaultAISettingsUseCase(userID))

	req := httptest.NewRequest(http.MethodGet, "/api/v3/users/me/ai-settings", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response AISettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Settings == nil {
		t.Fatal("expected settings in response")
	}
	if response.Settings.UserID != userID {
		t.Fatalf("unexpected user_id %v", response.Settings.UserID)
	}
	if response.Settings.AIMemoryEnabled {
		t.Fatal("ai_memory_enabled must default to false")
	}
	if response.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestAISettingsHandlerGetRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		c.Next() // no user set
	})
	NewAISettingsHandler(defaultAISettingsUseCase(uuid.New())).RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/users/me/ai-settings", nil)
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

func TestAISettingsHandlerPatchPartialUpdatePassesInput(t *testing.T) {
	userID := uuid.New()
	var captured service.UpdateAISettingsInput
	useCase := defaultAISettingsUseCase(userID)
	useCase.updateFn = func(_ context.Context, input service.UpdateAISettingsInput) (*entity.UserAISettings, error) {
		captured = input
		updated := entity.DefaultUserAISettings(userID)
		updated.Revision = input.ExpectedRevision + 1
		updated.AIMemoryEnabled = *input.AIMemoryEnabled
		return updated, nil
	}
	engine := newAISettingsTestEngine(userID, useCase)

	body := `{"expected_revision":0,"ai_memory_enabled":true}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if captured.UserID != userID {
		t.Fatalf("unexpected user_id %v", captured.UserID)
	}
	if captured.ExpectedRevision != 0 {
		t.Fatalf("expected revision 0, got %d", captured.ExpectedRevision)
	}
	if captured.AIMemoryEnabled == nil || !*captured.AIMemoryEnabled {
		t.Fatal("ai_memory_enabled must be passed as true")
	}
	if captured.AICompletionEnabled != nil ||
		captured.SpeechToTextEnabled != nil ||
		captured.CloudTextAllowed != nil ||
		captured.CloudAudioAllowed != nil {
		t.Fatalf("absent fields must stay nil, got %#v", captured)
	}

	var response AISettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !response.Settings.AIMemoryEnabled || response.Settings.Revision != 1 {
		t.Fatalf("unexpected updated settings: %#v", response.Settings)
	}
}

func TestAISettingsHandlerPatchInvalidJSON(t *testing.T) {
	userID := uuid.New()
	engine := newAISettingsTestEngine(userID, defaultAISettingsUseCase(userID))
	for _, body := range []string{"{", `{"expected_revision":"x"}`} {
		req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", strings.NewReader(body))
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

func TestAISettingsHandlerPatchMissingExpectedRevision(t *testing.T) {
	userID := uuid.New()
	engine := newAISettingsTestEngine(userID, defaultAISettingsUseCase(userID))
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", strings.NewReader(`{"ai_memory_enabled":true}`))
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

func TestAISettingsHandlerPatchNegativeExpectedRevision(t *testing.T) {
	userID := uuid.New()
	engine := newAISettingsTestEngine(userID, defaultAISettingsUseCase(userID))
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", strings.NewReader(`{"expected_revision":-1,"ai_memory_enabled":true}`))
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

func TestAISettingsHandlerPatchNoFields(t *testing.T) {
	userID := uuid.New()
	engine := newAISettingsTestEngine(userID, defaultAISettingsUseCase(userID))
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", strings.NewReader(`{"expected_revision":0}`))
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

func TestAISettingsHandlerPatchVersionConflict(t *testing.T) {
	userID := uuid.New()
	useCase := defaultAISettingsUseCase(userID)
	useCase.updateFn = func(context.Context, service.UpdateAISettingsInput) (*entity.UserAISettings, error) {
		return nil, service.ErrAISettingsConflict
	}
	engine := newAISettingsTestEngine(userID, useCase)
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", strings.NewReader(`{"expected_revision":0,"ai_memory_enabled":true}`))
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

func TestAISettingsHandlerPatchMapsInvalid(t *testing.T) {
	userID := uuid.New()
	useCase := defaultAISettingsUseCase(userID)
	useCase.updateFn = func(context.Context, service.UpdateAISettingsInput) (*entity.UserAISettings, error) {
		return nil, service.ErrInvalidAISettings
	}
	engine := newAISettingsTestEngine(userID, useCase)
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", strings.NewReader(`{"expected_revision":0,"ai_memory_enabled":true}`))
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

func TestAISettingsHandlerPatchAllFields(t *testing.T) {
	userID := uuid.New()
	useCase := defaultAISettingsUseCase(userID)
	engine := newAISettingsTestEngine(userID, useCase)

	body := bytes.NewBufferString(`{
		"expected_revision":3,
		"ai_memory_enabled":true,
		"ai_completion_enabled":true,
		"speech_to_text_enabled":true,
		"cloud_text_allowed":true,
		"cloud_audio_allowed":true
	}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/users/me/ai-settings", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response AISettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Settings.Revision != 4 {
		t.Fatalf("expected revision 4, got %d", response.Settings.Revision)
	}
	if !response.Settings.AIMemoryEnabled ||
		!response.Settings.AICompletionEnabled ||
		!response.Settings.SpeechToTextEnabled ||
		!response.Settings.CloudTextAllowed ||
		!response.Settings.CloudAudioAllowed {
		t.Fatalf("all fields should be enabled: %#v", response.Settings)
	}
}

func TestAISettingsHandlerGetIncludesPendingReorganize(t *testing.T) {
	userID := uuid.New()
	useCase := defaultAISettingsUseCase(userID)
	useCase.countFn = func(_ context.Context, got uuid.UUID) (int64, error) {
		if got != userID {
			t.Fatalf("unexpected user_id %v", got)
		}
		return 7, nil
	}
	engine := newAISettingsTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodGet, "/api/v3/users/me/ai-settings", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response AISettingsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.PendingReorganize != 7 {
		t.Fatalf("expected pending_reorganize 7, got %d", response.PendingReorganize)
	}
	if response.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestAISettingsHandlerReorganizeSuccess(t *testing.T) {
	userID := uuid.New()
	var calledWith uuid.UUID
	useCase := defaultAISettingsUseCase(userID)
	useCase.reorganizeFn = func(_ context.Context, got uuid.UUID) (int64, error) {
		calledWith = got
		return 12, nil
	}
	engine := newAISettingsTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/users/me/ai-settings/reorganize", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if calledWith != userID {
		t.Fatalf("reorganize called with %v, want %v", calledWith, userID)
	}
	var response ReorganizeResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Reorganized != 12 {
		t.Fatalf("expected reorganized 12, got %d", response.Reorganized)
	}
	if response.RequestID == "" {
		t.Fatal("expected request_id")
	}
}

func TestAISettingsHandlerReorganizeWhenAIMemoryDisabled(t *testing.T) {
	userID := uuid.New()
	useCase := defaultAISettingsUseCase(userID)
	useCase.reorganizeFn = func(context.Context, uuid.UUID) (int64, error) {
		return 0, service.ErrReorganizeAIMemoryDisabled
	}
	engine := newAISettingsTestEngine(userID, useCase)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/users/me/ai-settings/reorganize", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusConflict,
		V3ErrorFeatureNotEnabled,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestAISettingsHandlerReorganizeRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		c.Next() // no user set
	})
	NewAISettingsHandler(defaultAISettingsUseCase(uuid.New())).RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/users/me/ai-settings/reorganize", nil)
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
