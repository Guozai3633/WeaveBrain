package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/internal/stt"

	"github.com/google/uuid"
)

// ---- fakes ----

type audioAssetKey struct {
	userID  uuid.UUID
	assetID uuid.UUID
}

type fakeAudioAssetRepo struct {
	items map[audioAssetKey]*entity.AudioAsset
}

func newFakeAudioAssetRepo() *fakeAudioAssetRepo {
	return &fakeAudioAssetRepo{items: make(map[audioAssetKey]*entity.AudioAsset)}
}

func (r *fakeAudioAssetRepo) Initiate(_ context.Context, asset *entity.AudioAsset) error {
	key := audioAssetKey{asset.UserID, asset.ID}
	if _, ok := r.items[key]; ok {
		return nil // idempotent
	}
	cp := *asset
	cp.CreatedAt = time.Now().UTC()
	cp.UpdatedAt = time.Now().UTC()
	r.items[key] = &cp
	return nil
}

func (r *fakeAudioAssetRepo) GetByID(_ context.Context, userID, assetID uuid.UUID) (*entity.AudioAsset, error) {
	item, ok := r.items[audioAssetKey{userID, assetID}]
	if !ok {
		return nil, repository.ErrAudioAssetNotFound
	}
	return item, nil
}

func (r *fakeAudioAssetRepo) GetByCaptureID(_ context.Context, userID, captureID uuid.UUID) (*entity.AudioAsset, error) {
	for _, item := range r.items {
		if item.UserID == userID && item.CaptureID == captureID {
			return item, nil
		}
	}
	return nil, repository.ErrAudioAssetNotFound
}

func (r *fakeAudioAssetRepo) UpdateChunkProgress(_ context.Context, userID, assetID uuid.UUID, received int32) error {
	item, ok := r.items[audioAssetKey{userID, assetID}]
	if !ok {
		return repository.ErrAudioAssetNotFound
	}
	item.ReceivedChunks = received
	if item.UploadState != entity.AudioUploadStateComplete {
		item.UploadState = entity.AudioUploadStateUploading
	}
	return nil
}

func (r *fakeAudioAssetRepo) Complete(_ context.Context, userID, assetID uuid.UUID, sha string) error {
	item, ok := r.items[audioAssetKey{userID, assetID}]
	if !ok {
		return repository.ErrAudioAssetNotFound
	}
	item.UploadState = entity.AudioUploadStateComplete
	item.SHA256 = &sha
	item.ReceivedChunks = item.TotalChunks
	return nil
}

func (r *fakeAudioAssetRepo) MarkFailed(_ context.Context, userID, assetID uuid.UUID) error {
	item, ok := r.items[audioAssetKey{userID, assetID}]
	if !ok {
		return repository.ErrAudioAssetNotFound
	}
	item.UploadState = entity.AudioUploadStateFailed
	return nil
}

type transcriptKey struct {
	userID    uuid.UUID
	captureID uuid.UUID
}

type fakeTranscriptRepo struct {
	revisions map[transcriptKey][]*entity.TranscriptRevision
}

func newFakeTranscriptRepo() *fakeTranscriptRepo {
	return &fakeTranscriptRepo{revisions: make(map[transcriptKey][]*entity.TranscriptRevision)}
}

func (r *fakeTranscriptRepo) Append(_ context.Context, rev *entity.TranscriptRevision) error {
	key := transcriptKey{rev.UserID, rev.CaptureID}
	list := r.revisions[key]
	rev.Revision = int32(len(list)) + 1
	rev.CreatedAt = time.Now().UTC()
	r.revisions[key] = append(list, rev)
	return nil
}

func (r *fakeTranscriptRepo) GetLatest(_ context.Context, userID, captureID uuid.UUID) (*entity.TranscriptRevision, error) {
	list := r.revisions[transcriptKey{userID, captureID}]
	if len(list) == 0 {
		return nil, repository.ErrTranscriptNotFound
	}
	return list[len(list)-1], nil
}

func (r *fakeTranscriptRepo) ListByCapture(_ context.Context, userID, captureID uuid.UUID) ([]*entity.TranscriptRevision, error) {
	return r.revisions[transcriptKey{userID, captureID}], nil
}

type fakeSTTProvider struct {
	result stt.TranscriptionResult
	err    error
	calls  int
}

func (p *fakeSTTProvider) StreamRecognize(context.Context, <-chan []byte) (<-chan stt.TranscriptionResult, error) {
	panic("StreamRecognize is not used in audio service tests")
}

func (p *fakeSTTProvider) RecognizeFile(context.Context, string) (stt.TranscriptionResult, error) {
	p.calls++
	if p.err != nil {
		return stt.TranscriptionResult{}, p.err
	}
	return p.result, nil
}

func (p *fakeSTTProvider) Close() error { return nil }

// ---- helpers ----

func newAudioTestHarness(t *testing.T) (
	*AudioService,
	*memoryCaptureRepository,
	*fakeAudioAssetRepo,
	*fakeTranscriptRepo,
	*fakeSTTProvider,
) {
	t.Helper()
	captures := newMemoryCaptureRepository()
	assets := newFakeAudioAssetRepo()
	transcripts := newFakeTranscriptRepo()
	store, err := NewLocalAudioFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("init audio store: %v", err)
	}
	sttProvider := &fakeSTTProvider{}
	svc := NewAudioService(assets, transcripts, captures, store, sttProvider)
	return svc, captures, assets, transcripts, sttProvider
}

func createAudioCapture(t *testing.T, captures *memoryCaptureRepository, userID, captureID uuid.UUID) {
	t.Helper()
	if _, err := NewCaptureService(captures, nil).Create(context.Background(), userID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindAudio,
		Source:        "mobile_android",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create audio capture: %v", err)
	}
}

func createTextCapture(t *testing.T, captures *memoryCaptureRepository, userID, captureID uuid.UUID) {
	t.Helper()
	if _, err := NewCaptureService(captures, nil).Create(context.Background(), userID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindText,
		Text:          "hello text capture",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create text capture: %v", err)
	}
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func uploadAllChunks(t *testing.T, svc *AudioService, userID, assetID uuid.UUID, chunks [][]byte) {
	t.Helper()
	for i, chunk := range chunks {
		if err := svc.UploadChunk(context.Background(), userID, assetID, int32(i), hashBytes(chunk), chunk); err != nil {
			t.Fatalf("upload chunk %d: %v", i, err)
		}
	}
}

func splitChunks(data []byte, size int) [][]byte {
	var chunks [][]byte
	for len(data) > 0 {
		n := size
		if n > len(data) {
			n = len(data)
		}
		chunks = append(chunks, data[:n])
		data = data[n:]
	}
	return chunks
}

func defaultInitiateInput(assetID, captureID uuid.UUID, data []byte, totalChunks int32, sha string) InitiateAudioAssetInput {
	return InitiateAudioAssetInput{
		AssetID:     assetID,
		CaptureID:   captureID,
		MimeType:    "audio/wav",
		SizeBytes:   int64(len(data)),
		SHA256:      sha,
		TotalChunks: totalChunks,
		STTEnabled:  true,
	}
}

// ---- Initiate ----

func TestAudioServiceInitiateValid(t *testing.T) {
	svc, captures, assets, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	asset, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("x"), 2, strings.Repeat("a", 64)))
	if err != nil {
		t.Fatalf("initiate: %v", err)
	}
	if asset.UploadState != entity.AudioUploadStateInitiated {
		t.Fatalf("expected initiated, got %s", asset.UploadState)
	}
	if asset.TotalChunks != 2 {
		t.Fatalf("expected 2 total chunks, got %d", asset.TotalChunks)
	}
	if asset.StoragePath == "" {
		t.Fatalf("expected server-derived storage path")
	}
	if !strings.HasPrefix(asset.StoragePath, "audio/"+userID.String()) {
		t.Fatalf("storage path must be server-scoped to user, got %q", asset.StoragePath)
	}
	if asset.SHA256 == nil || *asset.SHA256 != strings.Repeat("a", 64) {
		t.Fatalf("sha256 not preserved")
	}
	if len(assets.items) != 1 {
		t.Fatalf("expected one stored asset, got %d", len(assets.items))
	}
}

func TestAudioServiceInitiateRejectsTextCapture(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createTextCapture(t, captures, userID, captureID)

	_, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("x"), 1, strings.Repeat("a", 64)))
	if !errors.Is(err, ErrInvalidAudioAsset) {
		t.Fatalf("expected invalid audio asset for text capture, got %v", err)
	}
}

func TestAudioServiceInitiateRejectsMissingCapture(t *testing.T) {
	svc, _, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()

	_, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("x"), 1, strings.Repeat("a", 64)))
	if !errors.Is(err, ErrInvalidAudioAsset) {
		t.Fatalf("expected invalid audio asset for missing capture, got %v", err)
	}
}

func TestAudioServiceInitiateIdempotent(t *testing.T) {
	svc, captures, assets, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)
	input := defaultInitiateInput(assetID, captureID, []byte("x"), 2, strings.Repeat("a", 64))

	if _, err := svc.Initiate(context.Background(), userID, input); err != nil {
		t.Fatalf("first initiate: %v", err)
	}
	second, err := svc.Initiate(context.Background(), userID, input)
	if err != nil {
		t.Fatalf("second initiate: %v", err)
	}
	if second.ID != assetID {
		t.Fatalf("idempotent initiate returned different asset")
	}
	if len(assets.items) != 1 {
		t.Fatalf("expected single asset after two initiates, got %d", len(assets.items))
	}
}

func TestAudioServiceInitiateValidation(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	valid := defaultInitiateInput(assetID, captureID, []byte("x"), 1, strings.Repeat("a", 64))
	longDur := int64(-1)

	cases := []struct {
		name  string
		user  uuid.UUID
		input InitiateAudioAssetInput
	}{
		{"nil user", uuid.Nil, valid},
		{"nil asset", userID, func() InitiateAudioAssetInput { v := valid; v.AssetID = uuid.Nil; return v }()},
		{"nil capture", userID, func() InitiateAudioAssetInput { v := valid; v.CaptureID = uuid.Nil; return v }()},
		{"empty mime", userID, func() InitiateAudioAssetInput { v := valid; v.MimeType = ""; return v }()},
		{"zero size", userID, func() InitiateAudioAssetInput { v := valid; v.SizeBytes = 0; return v }()},
		{"negative size", userID, func() InitiateAudioAssetInput { v := valid; v.SizeBytes = -5; return v }()},
		{"huge size", userID, func() InitiateAudioAssetInput { v := valid; v.SizeBytes = MaxAudioFileBytes + 1; return v }()},
		{"zero chunks", userID, func() InitiateAudioAssetInput { v := valid; v.TotalChunks = 0; return v }()},
		{"empty sha", userID, func() InitiateAudioAssetInput { v := valid; v.SHA256 = ""; return v }()},
		{"short sha", userID, func() InitiateAudioAssetInput { v := valid; v.SHA256 = "abcd"; return v }()},
		{"negative duration", userID, func() InitiateAudioAssetInput { v := valid; v.DurationMs = &longDur; return v }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Initiate(context.Background(), tc.user, tc.input)
			if !errors.Is(err, ErrInvalidAudioAsset) {
				t.Fatalf("expected invalid audio asset, got %v", err)
			}
		})
	}
}

// ---- UploadChunk ----

func TestAudioServiceUploadChunkChecksumMismatch(t *testing.T) {
	svc, captures, assets, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("hello world"), 1, strings.Repeat("a", 64))); err != nil {
		t.Fatalf("initiate: %v", err)
	}

	err := svc.UploadChunk(context.Background(), userID, assetID, 0, strings.Repeat("0", 64), []byte("hello world"))
	if !errors.Is(err, ErrAudioChecksumMismatch) {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
	if got := assets.items[audioAssetKey{userID, assetID}].ReceivedChunks; got != 0 {
		t.Fatalf("mismatched chunk must not be counted, got %d", got)
	}
}

func TestAudioServiceUploadChunkOutOfRange(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("hello world"), 2, strings.Repeat("a", 64))); err != nil {
		t.Fatalf("initiate: %v", err)
	}

	for _, idx := range []int32{-1, 2, 5} {
		err := svc.UploadChunk(context.Background(), userID, assetID, idx, hashBytes([]byte("x")), []byte("x"))
		if !errors.Is(err, ErrAudioChunkOutOfRange) {
			t.Fatalf("index %d: expected out of range, got %v", idx, err)
		}
	}
}

func TestAudioServiceUploadChunkTooLarge(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("hello world"), 1, strings.Repeat("a", 64))); err != nil {
		t.Fatalf("initiate: %v", err)
	}

	oversized := make([]byte, MaxAudioChunkBytes+1)
	err := svc.UploadChunk(context.Background(), userID, assetID, 0, hashBytes(oversized), oversized)
	if !errors.Is(err, ErrAudioChunkTooLarge) {
		t.Fatalf("expected chunk too large, got %v", err)
	}
}

func TestAudioServiceUploadChunkNotFound(t *testing.T) {
	svc, _, _, _, _ := newAudioTestHarness(t)
	userID, assetID := uuid.New(), uuid.New()

	err := svc.UploadChunk(context.Background(), userID, assetID, 0, hashBytes([]byte("x")), []byte("x"))
	if !errors.Is(err, ErrAudioAssetNotFound) {
		t.Fatalf("expected asset not found, got %v", err)
	}
}

func TestAudioServiceUploadChunkRetryIdempotent(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("hello world"), 1, strings.Repeat("a", 64))); err != nil {
		t.Fatalf("initiate: %v", err)
	}

	chunk := []byte("part0")
	for i := 0; i < 3; i++ {
		if err := svc.UploadChunk(context.Background(), userID, assetID, 0, hashBytes(chunk), chunk); err != nil {
			t.Fatalf("retry %d: %v", i, err)
		}
	}
	stored, err := svc.GetByID(context.Background(), userID, assetID)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if stored.ReceivedChunks != 1 {
		t.Fatalf("retry must not double-count chunk, got %d", stored.ReceivedChunks)
	}
}

// ---- Complete ----

func TestAudioServiceCompleteSuccess(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("this is a complete wav file body")
	chunks := splitChunks(audio, 5)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)

	asset, err := svc.Complete(context.Background(), userID, assetID)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if asset.UploadState != entity.AudioUploadStateComplete {
		t.Fatalf("expected complete, got %s", asset.UploadState)
	}
	if asset.SHA256 == nil || *asset.SHA256 != hashBytes(audio) {
		t.Fatalf("full-file sha mismatch")
	}
	if asset.ReceivedChunks != int32(len(chunks)) {
		t.Fatalf("expected %d received chunks, got %d", len(chunks), asset.ReceivedChunks)
	}
}

func TestAudioServiceCompleteMissingChunks(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("hello world")
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, 2, hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	chunks := splitChunks(audio, 5) // 3 chunks declared as 2 -> out of range; use only first 2 chunks of 2 declared
	uploadAllChunks(t, svc, userID, assetID, chunks[:1])

	_, err := svc.Complete(context.Background(), userID, assetID)
	if !errors.Is(err, ErrAudioUploadNotReady) {
		t.Fatalf("expected upload not ready, got %v", err)
	}
}

func TestAudioServiceCompleteOutOfOrderChunks(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("hello world")
	chunks := splitChunks(audio, 5) // [hello][ worl][d] -> 3 chunks
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks")
	}
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, 2, hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	// Upload only chunk index 1 (missing chunk 0).
	if err := svc.UploadChunk(context.Background(), userID, assetID, 1, hashBytes(chunks[1]), chunks[1]); err != nil {
		t.Fatalf("upload chunk 1: %v", err)
	}
	_, err := svc.Complete(context.Background(), userID, assetID)
	if !errors.Is(err, ErrAudioUploadNotReady) {
		t.Fatalf("expected upload not ready for out-of-order chunks, got %v", err)
	}
}

func TestAudioServiceCompleteSizeMismatch(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("hello world")
	chunks := splitChunks(audio, 5)
	input := defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))
	input.SizeBytes = int64(len(audio)) + 100 // declared larger than actual
	if _, err := svc.Initiate(context.Background(), userID, input); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)

	_, err := svc.Complete(context.Background(), userID, assetID)
	if !errors.Is(err, ErrAudioChecksumMismatch) {
		t.Fatalf("expected size mismatch -> checksum mismatch, got %v", err)
	}
}

func TestAudioServiceCompleteFullFileSHA256Mismatch(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("hello world")
	chunks := splitChunks(audio, 5)
	input := defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), strings.Repeat("f", 64))
	if _, err := svc.Initiate(context.Background(), userID, input); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)

	_, err := svc.Complete(context.Background(), userID, assetID)
	if !errors.Is(err, ErrAudioChecksumMismatch) {
		t.Fatalf("expected full-file sha mismatch, got %v", err)
	}
}

func TestAudioServiceCompleteIdempotent(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("hello world")
	chunks := splitChunks(audio, 5)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)

	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("first complete: %v", err)
	}
	second, err := svc.Complete(context.Background(), userID, assetID)
	if err != nil {
		t.Fatalf("second complete: %v", err)
	}
	if second.UploadState != entity.AudioUploadStateComplete {
		t.Fatalf("expected complete on idempotent re-complete, got %s", second.UploadState)
	}
}

func TestAudioServiceCompleteNoChunks(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("x"), 1, strings.Repeat("a", 64))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	_, err := svc.Complete(context.Background(), userID, assetID)
	if !errors.Is(err, ErrAudioUploadNotReady) {
		t.Fatalf("expected upload not ready, got %v", err)
	}
}

// ---- TranscribeCapture ----

func TestAudioServiceTranscribeCaptureSuccess(t *testing.T) {
	svc, captures, _, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("audio bytes for transcription")
	chunks := splitChunks(audio, 6)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)
	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	sttP.result = stt.TranscriptionResult{Text: "跑步时想到的一个好主意", IsFinal: true, Confidence: 0.95}
	rev, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if rev.Source != entity.TranscriptSourceSTT {
		t.Fatalf("expected stt source, got %s", rev.Source)
	}
	if rev.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", rev.Revision)
	}
	if rev.Text != "跑步时想到的一个好主意" {
		t.Fatalf("unexpected transcript text")
	}
	if got := captures.items[captureKey{userID, captureID}].MemoryCard.Title; got != fallbackTitle("跑步时想到的一个好主意") {
		t.Fatalf("expected card title updated to transcript, got %q", got)
	}
}

func TestAudioServiceTranscribeCaptureNotReady(t *testing.T) {
	svc, captures, _, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("x"), 1, strings.Repeat("a", 64))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	_, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if !errors.Is(err, ErrAudioUploadNotReady) {
		t.Fatalf("expected upload not ready, got %v", err)
	}
	if sttP.calls != 0 {
		t.Fatalf("STT must not be called for incomplete upload, got %d calls", sttP.calls)
	}
}

func TestAudioServiceTranscribeCaptureSTTDisabled(t *testing.T) {
	svc, captures, _, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("audio")
	chunks := splitChunks(audio, 3)
	input := defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))
	input.STTEnabled = false
	if _, err := svc.Initiate(context.Background(), userID, input); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)
	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	_, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if !errors.Is(err, ErrInvalidAudioAsset) {
		t.Fatalf("expected invalid audio asset when STT disabled, got %v", err)
	}
	if sttP.calls != 0 {
		t.Fatalf("STT must not be called when disabled, got %d calls", sttP.calls)
	}
}

func TestAudioServiceTranscribeCaptureNoAudio(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID := uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	_, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if !errors.Is(err, ErrAudioAssetNotFound) {
		t.Fatalf("expected audio asset not found, got %v", err)
	}
}

func TestAudioServiceTranscribeFailurePreservesAudio(t *testing.T) {
	svc, captures, assetsRepo, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("audio body")
	chunks := splitChunks(audio, 4)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)
	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	sttP.err = errors.New("stt service down")
	_, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if err == nil {
		t.Fatalf("expected STT failure")
	}
	// Original audio must still exist and be complete (never deleted on failure).
	stored, err := svc.GetByID(context.Background(), userID, assetID)
	if err != nil {
		t.Fatalf("audio asset should still be readable: %v", err)
	}
	if stored.UploadState != entity.AudioUploadStateComplete {
		t.Fatalf("audio asset must remain complete after STT failure, got %s", stored.UploadState)
	}
	if stored.SHA256 == nil || *stored.SHA256 != hashBytes(audio) {
		t.Fatalf("audio checksum must be preserved after STT failure")
	}
	if len(assetsRepo.items) != 1 {
		t.Fatalf("audio asset must not be deleted, got %d items", len(assetsRepo.items))
	}
}

func TestAudioServiceTranscribeEmptyResult(t *testing.T) {
	svc, captures, _, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("audio body")
	chunks := splitChunks(audio, 4)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)
	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	sttP.result = stt.TranscriptionResult{Text: "   "}
	_, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if err == nil {
		t.Fatalf("expected error for empty STT transcript")
	}
}

func TestAudioServiceTranscribeRevisionIncrement(t *testing.T) {
	svc, captures, _, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("audio body")
	chunks := splitChunks(audio, 4)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)
	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("complete: %v", err)
	}

	sttP.result = stt.TranscriptionResult{Text: "first pass", IsFinal: true}
	first, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("first transcribe: %v", err)
	}
	sttP.result = stt.TranscriptionResult{Text: "second pass", IsFinal: true}
	second, err := svc.TranscribeCapture(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("second transcribe: %v", err)
	}
	if first.Revision != 1 || second.Revision != 2 {
		t.Fatalf("expected revisions 1,2 got %d,%d", first.Revision, second.Revision)
	}
}

// ---- CorrectTranscript ----

func TestAudioServiceCorrectTranscriptSuccess(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID := uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	rev, err := svc.CorrectTranscript(context.Background(), userID, captureID, "我修正后的转写内容")
	if err != nil {
		t.Fatalf("correct transcript: %v", err)
	}
	if rev.Source != entity.TranscriptSourceUser {
		t.Fatalf("expected user source, got %s", rev.Source)
	}
	if rev.Revision != 1 {
		t.Fatalf("expected revision 1, got %d", rev.Revision)
	}
	if rev.Confidence != nil {
		t.Fatalf("user correction must not carry confidence")
	}
	if got := captures.items[captureKey{userID, captureID}].MemoryCard.Title; got != fallbackTitle("我修正后的转写内容") {
		t.Fatalf("expected card title updated, got %q", got)
	}
}

func TestAudioServiceCorrectTranscriptValidation(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID := uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	cases := []struct {
		name string
		text string
	}{
		{"empty", "   "},
		{"too long", strings.Repeat("字", MaxTranscriptRunes+1)},
		{"invalid utf8", string([]byte{0xff, 0xfe, 0xfd})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CorrectTranscript(context.Background(), userID, captureID, tc.text)
			if !errors.Is(err, ErrInvalidTranscript) {
				t.Fatalf("expected invalid transcript, got %v", err)
			}
		})
	}
}

func TestAudioServiceCorrectTranscriptAfterSTTIncrementsRevision(t *testing.T) {
	svc, captures, _, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("audio body")
	chunks := splitChunks(audio, 4)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)
	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	sttP.result = stt.TranscriptionResult{Text: "machine transcript", IsFinal: true}
	if _, err := svc.TranscribeCapture(context.Background(), userID, captureID); err != nil {
		t.Fatalf("transcribe: %v", err)
	}

	userRev, err := svc.CorrectTranscript(context.Background(), userID, captureID, "human corrected")
	if err != nil {
		t.Fatalf("correct: %v", err)
	}
	if userRev.Revision != 2 {
		t.Fatalf("expected user revision 2 after stt revision 1, got %d", userRev.Revision)
	}
}

// ---- GetByCapture / GetByID / GetLatestTranscript ----

func TestAudioServiceGetByCaptureAudioDetail(t *testing.T) {
	svc, captures, _, _, sttP := newAudioTestHarness(t)
	userID, captureID, assetID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)

	audio := []byte("audio body")
	chunks := splitChunks(audio, 4)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, audio, int32(len(chunks)), hashBytes(audio))); err != nil {
		t.Fatalf("initiate: %v", err)
	}
	uploadAllChunks(t, svc, userID, assetID, chunks)
	if _, err := svc.Complete(context.Background(), userID, assetID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	sttP.result = stt.TranscriptionResult{Text: "transcript text", IsFinal: true}
	if _, err := svc.TranscribeCapture(context.Background(), userID, captureID); err != nil {
		t.Fatalf("transcribe: %v", err)
	}

	agg, err := svc.GetByCapture(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("get by capture: %v", err)
	}
	if agg.Audio == nil || agg.Audio.ID != assetID {
		t.Fatalf("expected audio asset in aggregate")
	}
	if agg.Transcript == nil || agg.Transcript.Text != "transcript text" {
		t.Fatalf("expected transcript in aggregate")
	}
}

func TestAudioServiceGetByCaptureTextNoAudio(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, captureID := uuid.New(), uuid.New()
	createTextCapture(t, captures, userID, captureID)

	agg, err := svc.GetByCapture(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("get by capture: %v", err)
	}
	if agg.Audio != nil || agg.Transcript != nil {
		t.Fatalf("text capture must have nil audio/transcript")
	}
}

func TestAudioServiceGetByCaptureUserScoped(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	ownerID, otherID, captureID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, ownerID, captureID)

	if _, err := svc.GetByCapture(context.Background(), otherID, captureID); !errors.Is(err, ErrCaptureNotFound) {
		t.Fatalf("other user must see not found, got %v", err)
	}
}

func TestAudioServiceGetByIDUserScoped(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	userID, otherID, captureID, assetID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, userID, captureID)
	if _, err := svc.Initiate(context.Background(), userID, defaultInitiateInput(assetID, captureID, []byte("x"), 1, strings.Repeat("a", 64))); err != nil {
		t.Fatalf("initiate: %v", err)
	}

	if _, err := svc.GetByID(context.Background(), otherID, assetID); !errors.Is(err, ErrAudioAssetNotFound) {
		t.Fatalf("other user must see not found, got %v", err)
	}
}

func TestAudioServiceGetLatestTranscriptUserScoped(t *testing.T) {
	svc, captures, _, _, _ := newAudioTestHarness(t)
	ownerID, otherID, captureID := uuid.New(), uuid.New(), uuid.New()
	createAudioCapture(t, captures, ownerID, captureID)
	if _, err := svc.CorrectTranscript(context.Background(), ownerID, captureID, "mine"); err != nil {
		t.Fatalf("correct: %v", err)
	}
	if _, err := svc.GetLatestTranscript(context.Background(), otherID, captureID); !errors.Is(err, ErrAudioAssetNotFound) {
		t.Fatalf("other user must not read transcript, got %v", err)
	}
}
