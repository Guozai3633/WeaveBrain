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

type fakeAudioUseCase struct {
	initiateFn            func(context.Context, uuid.UUID, service.InitiateAudioAssetInput) (*entity.AudioAsset, error)
	getByIDFn             func(context.Context, uuid.UUID, uuid.UUID) (*entity.AudioAsset, error)
	getByCaptureFn        func(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAudioAggregate, error)
	uploadChunkFn         func(context.Context, uuid.UUID, uuid.UUID, int32, string, []byte) error
	completeFn            func(context.Context, uuid.UUID, uuid.UUID) (*entity.AudioAsset, error)
	transcribeFn          func(context.Context, uuid.UUID, uuid.UUID) (*entity.TranscriptRevision, error)
	correctTranscriptFn   func(context.Context, uuid.UUID, uuid.UUID, string) (*entity.TranscriptRevision, error)
	getLatestTranscriptFn func(context.Context, uuid.UUID, uuid.UUID) (*entity.TranscriptRevision, error)
}

func (f *fakeAudioUseCase) Initiate(ctx context.Context, u uuid.UUID, i service.InitiateAudioAssetInput) (*entity.AudioAsset, error) {
	return f.initiateFn(ctx, u, i)
}
func (f *fakeAudioUseCase) GetByID(ctx context.Context, u uuid.UUID, a uuid.UUID) (*entity.AudioAsset, error) {
	return f.getByIDFn(ctx, u, a)
}
func (f *fakeAudioUseCase) GetByCapture(ctx context.Context, u uuid.UUID, c uuid.UUID) (*entity.CaptureAudioAggregate, error) {
	return f.getByCaptureFn(ctx, u, c)
}
func (f *fakeAudioUseCase) UploadChunk(ctx context.Context, u uuid.UUID, a uuid.UUID, i int32, h string, d []byte) error {
	return f.uploadChunkFn(ctx, u, a, i, h, d)
}
func (f *fakeAudioUseCase) Complete(ctx context.Context, u uuid.UUID, a uuid.UUID) (*entity.AudioAsset, error) {
	return f.completeFn(ctx, u, a)
}
func (f *fakeAudioUseCase) TranscribeCapture(ctx context.Context, u uuid.UUID, c uuid.UUID) (*entity.TranscriptRevision, error) {
	return f.transcribeFn(ctx, u, c)
}
func (f *fakeAudioUseCase) CorrectTranscript(ctx context.Context, u uuid.UUID, c uuid.UUID, t string) (*entity.TranscriptRevision, error) {
	return f.correctTranscriptFn(ctx, u, c, t)
}
func (f *fakeAudioUseCase) GetLatestTranscript(ctx context.Context, u uuid.UUID, c uuid.UUID) (*entity.TranscriptRevision, error) {
	return f.getLatestTranscriptFn(ctx, u, c)
}

func newAudioHandlerTestEngine(userID uuid.UUID, useCase audioUseCase) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		SetCurrentUserID(c, userID)
		c.Next()
	})
	NewAudioAssetHandler(useCase).RegisterRoutes(group)
	return engine
}

func audioAssetFixture(userID, captureID, assetID uuid.UUID, state entity.AudioUploadState) *entity.AudioAsset {
	now := time.Now().UTC()
	sha := strings.Repeat("a", 64)
	return &entity.AudioAsset{
		ID:             assetID,
		UserID:         userID,
		CaptureID:      captureID,
		MimeType:       "audio/wav",
		SizeBytes:      1024,
		SHA256:         &sha,
		StoragePath:    "audio/" + userID.String() + "/" + assetID.String() + ".wav",
		UploadState:    state,
		TotalChunks:    1,
		ReceivedChunks: 1,
		STTEnabled:     true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func defaultAudioUseCase(userID, captureID, assetID uuid.UUID) *fakeAudioUseCase {
	asset := audioAssetFixture(userID, captureID, assetID, entity.AudioUploadStateComplete)
	transcript := &entity.TranscriptRevision{
		ID:        1,
		UserID:    userID,
		CaptureID: captureID,
		Revision:  1,
		Text:      "转写结果",
		Source:    entity.TranscriptSourceSTT,
	}
	return &fakeAudioUseCase{
		initiateFn: func(_ context.Context, got uuid.UUID, _ service.InitiateAudioAssetInput) (*entity.AudioAsset, error) {
			if got != userID {
				return nil, service.ErrInvalidAudioAsset
			}
			return asset, nil
		},
		getByIDFn: func(_ context.Context, got uuid.UUID, gotID uuid.UUID) (*entity.AudioAsset, error) {
			if got != userID || gotID != assetID {
				return nil, service.ErrAudioAssetNotFound
			}
			return asset, nil
		},
		getByCaptureFn: func(_ context.Context, got uuid.UUID, gotID uuid.UUID) (*entity.CaptureAudioAggregate, error) {
			if got != userID || gotID != captureID {
				return nil, service.ErrCaptureNotFound
			}
			return &entity.CaptureAudioAggregate{Capture: &entity.Capture{ID: captureID}, MemoryCard: &entity.MemoryCard{CaptureID: captureID}, Audio: asset, Transcript: transcript}, nil
		},
		uploadChunkFn: func(_ context.Context, got uuid.UUID, gotID uuid.UUID, _ int32, _ string, _ []byte) error {
			if got != userID || gotID != assetID {
				return service.ErrAudioAssetNotFound
			}
			return nil
		},
		completeFn: func(_ context.Context, got uuid.UUID, gotID uuid.UUID) (*entity.AudioAsset, error) {
			if got != userID || gotID != assetID {
				return nil, service.ErrAudioAssetNotFound
			}
			return asset, nil
		},
		transcribeFn: func(_ context.Context, got uuid.UUID, gotID uuid.UUID) (*entity.TranscriptRevision, error) {
			if got != userID || gotID != captureID {
				return nil, service.ErrCaptureNotFound
			}
			return transcript, nil
		},
		correctTranscriptFn: func(_ context.Context, got uuid.UUID, gotID uuid.UUID, _ string) (*entity.TranscriptRevision, error) {
			if got != userID || gotID != captureID {
				return nil, service.ErrCaptureNotFound
			}
			return transcript, nil
		},
		getLatestTranscriptFn: func(_ context.Context, got uuid.UUID, gotID uuid.UUID) (*entity.TranscriptRevision, error) {
			if got != userID || gotID != captureID {
				return nil, service.ErrAudioAssetNotFound
			}
			return transcript, nil
		},
	}
}

func TestAudioHandlerInitiate(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))
	body := []byte(`{
		"asset_id":"` + assetID.String() + `",
		"capture_id":"` + captureID.String() + `",
		"mime_type":"audio/wav",
		"size_bytes":1024,
		"sha256":"` + strings.Repeat("a", 64) + `",
		"total_chunks":1,
		"stt_enabled":true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v3/audio-assets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var response AudioAssetResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.Asset.ID != assetID {
		t.Fatalf("unexpected asset in response")
	}
}

func TestAudioHandlerInitiateBadRequest(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))

	for _, body := range []string{
		`{`,
		`{"asset_id":"not-a-uuid"}`,
		`{"capture_id":"not-a-uuid"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v3/audio-assets", strings.NewReader(body))
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

func TestAudioHandlerUploadChunkValid(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))
	req := httptest.NewRequest(http.MethodPut, "/api/v3/audio-assets/"+assetID.String()+"/chunks/0", strings.NewReader("binary audio bytes"))
	req.Header.Set(chunkSHA256Header, strings.Repeat("a", 64))
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAudioHandlerUploadChunkInvalidSHAHeader(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))

	for _, h := range []string{"", "zz", strings.Repeat("a", 63)} {
		req := httptest.NewRequest(http.MethodPut, "/api/v3/audio-assets/"+assetID.String()+"/chunks/0", strings.NewReader("data"))
		if h != "" {
			req.Header.Set(chunkSHA256Header, h)
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

func TestAudioHandlerUploadChunkInvalidIndex(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))
	req := httptest.NewRequest(http.MethodPut, "/api/v3/audio-assets/"+assetID.String()+"/chunks/abc", strings.NewReader("data"))
	req.Header.Set(chunkSHA256Header, strings.Repeat("a", 64))
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

func TestAudioHandlerUploadChunkMapsChecksumMismatch(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	useCase := defaultAudioUseCase(userID, captureID, assetID)
	useCase.uploadChunkFn = func(context.Context, uuid.UUID, uuid.UUID, int32, string, []byte) error {
		return service.ErrAudioChecksumMismatch
	}
	engine := newAudioHandlerTestEngine(userID, useCase)
	req := httptest.NewRequest(http.MethodPut, "/api/v3/audio-assets/"+assetID.String()+"/chunks/0", strings.NewReader("data"))
	req.Header.Set(chunkSHA256Header, strings.Repeat("a", 64))
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

func TestAudioHandlerCompleteSuccessAndPrecondition(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	completeCalled := 0

	useCase := defaultAudioUseCase(userID, captureID, assetID)
	useCase.completeFn = func(context.Context, uuid.UUID, uuid.UUID) (*entity.AudioAsset, error) {
		completeCalled++
		if completeCalled == 1 {
			return nil, service.ErrAudioUploadNotReady
		}
		return audioAssetFixture(userID, captureID, assetID, entity.AudioUploadStateComplete), nil
	}
	engine := newAudioHandlerTestEngine(userID, useCase)

	// First complete -> precondition failed.
	req := httptest.NewRequest(http.MethodPost, "/api/v3/audio-assets/"+assetID.String()+"/complete", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusPreconditionFailed,
		V3ErrorPreconditionFailed,
		recorder.Header().Get(requestIDHeader),
	)

	// Second complete -> success.
	req = httptest.NewRequest(http.MethodPost, "/api/v3/audio-assets/"+assetID.String()+"/complete", nil)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAudioHandlerTranscribeAndCorrect(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))

	// Transcribe.
	req := httptest.NewRequest(http.MethodPost, "/api/v3/captures/"+captureID.String()+"/transcribe", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}

	// Correct transcript.
	req = httptest.NewRequest(http.MethodPatch, "/api/v3/captures/"+captureID.String()+"/transcript", strings.NewReader(`{"text":"修正后转写"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAudioHandlerCorrectTranscriptInvalid(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	useCase := defaultAudioUseCase(userID, captureID, assetID)
	useCase.correctTranscriptFn = func(context.Context, uuid.UUID, uuid.UUID, string) (*entity.TranscriptRevision, error) {
		return nil, service.ErrInvalidTranscript
	}
	engine := newAudioHandlerTestEngine(userID, useCase)
	req := httptest.NewRequest(http.MethodPatch, "/api/v3/captures/"+captureID.String()+"/transcript", strings.NewReader(`{"text":""}`))
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

func TestAudioHandlerGetByCaptureAndByID(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))

	// By capture.
	req := httptest.NewRequest(http.MethodGet, "/api/v3/audio-assets/by-capture/"+captureID.String(), nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var detail AudioDetailResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if detail.Audio == nil || detail.Transcript == nil {
		t.Fatalf("expected audio + transcript in detail response")
	}

	// By asset id.
	req = httptest.NewRequest(http.MethodGet, "/api/v3/audio-assets/"+assetID.String(), nil)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	// Get transcript.
	req = httptest.NewRequest(http.MethodGet, "/api/v3/captures/"+captureID.String()+"/transcript", nil)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAudioHandlerMapsNotFound(t *testing.T) {
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	engine := newAudioHandlerTestEngine(userID, defaultAudioUseCase(userID, captureID, assetID))

	// GET by asset id for a non-owned asset returns 404.
	req := httptest.NewRequest(http.MethodGet, "/api/v3/audio-assets/"+uuid.NewString(), nil)
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

func TestAudioHandlerRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	group.Use(func(c *gin.Context) {
		// No user set -> authentication failure.
		c.Next()
	})
	NewAudioAssetHandler(defaultAudioUseCase(uuid.New(), uuid.New(), uuid.New())).RegisterRoutes(group)

	req := httptest.NewRequest(http.MethodPost, "/api/v3/audio-assets", strings.NewReader(`{}`))
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
