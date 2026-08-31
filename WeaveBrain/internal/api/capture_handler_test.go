package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"weavebrain/internal/entity"
	"weavebrain/internal/service"
	"weavebrain/pkg/auth"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeCaptureUseCase struct {
	createFn func(context.Context, uuid.UUID, service.CreateCaptureInput) (*service.CreateCaptureResult, error)
	getFn    func(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAggregate, error)
}

func (f *fakeCaptureUseCase) Create(
	ctx context.Context,
	userID uuid.UUID,
	input service.CreateCaptureInput,
) (*service.CreateCaptureResult, error) {
	return f.createFn(ctx, userID, input)
}

func (f *fakeCaptureUseCase) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CaptureAggregate, error) {
	return f.getFn(ctx, userID, captureID)
}

func newCaptureHandlerTestEngine(userID uuid.UUID, useCase captureUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		SetCurrentUserID(c, userID)
		c.Next()
	})
	NewCaptureHandler(useCase).RegisterRoutes(group)
	return engine
}

func captureAggregate(userID, captureID uuid.UUID, text string) *entity.CaptureAggregate {
	now := time.Now().UTC()
	return &entity.CaptureAggregate{
		Capture: &entity.Capture{
			ID:                  captureID,
			UserID:              userID,
			Kind:                entity.CaptureKindText,
			OriginalText:        &text,
			CapturedAtPrecision: "unknown",
			Source:              "web",
			PrivacyMode:         "cloud_allowed",
			ClientVersion:       1,
			Version:             1,
			LifecycleStatus:     "active",
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		MemoryCard: &entity.MemoryCard{
			ID:               uuid.New(),
			UserID:           userID,
			CaptureID:        captureID,
			PrimaryType:      "uncategorized",
			Title:            text,
			ProcessingStatus: "ready",
			Version:          1,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}
}

func TestCaptureHandlerCreateAndReplayContracts(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	replayed := false
	useCase := &fakeCaptureUseCase{
		createFn: func(
			_ context.Context,
			gotUserID uuid.UUID,
			input service.CreateCaptureInput,
		) (*service.CreateCaptureResult, error) {
			if gotUserID != userID {
				t.Fatalf("expected user %s, got %s", userID, gotUserID)
			}
			if input.ID != captureID || input.CollectionID != nil {
				t.Fatalf("unexpected capture input: %#v", input)
			}
			return &service.CreateCaptureResult{
				Aggregate: captureAggregate(userID, captureID, input.Text),
				Replayed:  replayed,
			}, nil
		},
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAggregate, error) {
			panic("unexpected get")
		},
	}
	engine := newCaptureHandlerTestEngine(userID, useCase)
	body := []byte(`{
		"capture_id":"` + captureID.String() + `",
		"kind":"text",
		"text":"idea without project",
		"source":"web",
		"client_version":1
	}`)

	for attempt, expectedStatus := range []int{http.StatusCreated, http.StatusOK} {
		replayed = attempt > 0
		req := httptest.NewRequest(http.MethodPost, "/api/v3/captures", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(idempotencyKeyHeader, captureID.String())
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, req)

		if recorder.Code != expectedStatus {
			t.Fatalf("attempt %d: expected status %d, got %d: %s", attempt+1, expectedStatus, recorder.Code, recorder.Body.String())
		}
		var response CaptureResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.Capture.ID != captureID || response.MemoryCard.CaptureID != captureID {
			t.Fatalf("response does not contain requested aggregate: %#v", response)
		}
		if response.RequestID != recorder.Header().Get(requestIDHeader) {
			t.Fatalf("request_id body/header mismatch")
		}
		if response.Replayed != replayed {
			t.Fatalf("expected replayed=%v, got %v", replayed, response.Replayed)
		}
	}
}

func TestCaptureHandlerRejectsInvalidIdempotencyKey(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	called := false
	useCase := &fakeCaptureUseCase{
		createFn: func(context.Context, uuid.UUID, service.CreateCaptureInput) (*service.CreateCaptureResult, error) {
			called = true
			return nil, nil
		},
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAggregate, error) {
			return nil, service.ErrCaptureNotFound
		},
	}
	engine := newCaptureHandlerTestEngine(userID, useCase)
	body := []byte(`{"capture_id":"` + captureID.String() + `","kind":"text","text":"idea","client_version":1}`)

	tests := []struct {
		name string
		key  string
	}{
		{name: "missing"},
		{name: "invalid", key: "not-a-uuid"},
		{name: "mismatch", key: uuid.NewString()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodPost, "/api/v3/captures", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			if test.key != "" {
				req.Header.Set(idempotencyKeyHeader, test.key)
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
			if called {
				t.Fatal("service must not run for invalid idempotency contract")
			}
		})
	}
}

func TestCaptureHandlerMapsIdempotencyConflict(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	useCase := &fakeCaptureUseCase{
		createFn: func(context.Context, uuid.UUID, service.CreateCaptureInput) (*service.CreateCaptureResult, error) {
			return nil, service.ErrCaptureIdempotencyConflict
		},
		getFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAggregate, error) {
			return nil, service.ErrCaptureNotFound
		},
	}
	engine := newCaptureHandlerTestEngine(userID, useCase)
	body := []byte(`{"capture_id":"` + captureID.String() + `","kind":"text","text":"different","client_version":1}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v3/captures", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(idempotencyKeyHeader, captureID.String())
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusConflict,
		V3ErrorIdempotencyConflict,
		recorder.Header().Get(requestIDHeader),
	)
}

func TestCaptureHandlerGetUsesAuthenticatedUserAndHidesOtherUsers(t *testing.T) {
	ownerID := uuid.New()
	otherUserID := uuid.New()
	captureID := uuid.New()
	useCase := &fakeCaptureUseCase{
		createFn: func(context.Context, uuid.UUID, service.CreateCaptureInput) (*service.CreateCaptureResult, error) {
			panic("unexpected create")
		},
		getFn: func(_ context.Context, userID uuid.UUID, gotCaptureID uuid.UUID) (*entity.CaptureAggregate, error) {
			if gotCaptureID != captureID {
				t.Fatalf("expected capture %s, got %s", captureID, gotCaptureID)
			}
			if userID != ownerID {
				return nil, service.ErrCaptureNotFound
			}
			return captureAggregate(ownerID, captureID, "private"), nil
		},
	}

	for _, test := range []struct {
		name           string
		userID         uuid.UUID
		expectedStatus int
	}{
		{name: "owner", userID: ownerID, expectedStatus: http.StatusOK},
		{name: "other user", userID: otherUserID, expectedStatus: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := newCaptureHandlerTestEngine(test.userID, useCase)
			req := httptest.NewRequest(http.MethodGet, "/api/v3/captures/"+captureID.String(), nil)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)
			if recorder.Code != test.expectedStatus {
				t.Fatalf("expected status %d, got %d: %s", test.expectedStatus, recorder.Code, recorder.Body.String())
			}
			if test.expectedStatus == http.StatusNotFound {
				assertV3ErrorResponse(
					t,
					recorder,
					http.StatusNotFound,
					V3ErrorNotFound,
					recorder.Header().Get(requestIDHeader),
				)
			}
		})
	}
}

func TestV3JWTMiddlewareUsesV3ErrorContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(v3JWTMiddleware(auth.TokenConfig{
		Secret: []byte("test-secret"),
		Issuer: "test",
		Expiry: time.Hour,
	}))
	group.GET("/protected", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v3/protected", nil)
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
