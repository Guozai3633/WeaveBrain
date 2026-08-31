package service

import (
	"context"
	"os"
	"testing"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/internal/stt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// r7SttProvider returns a fixed transcript and counts invocations.
type r7SttProvider struct {
	text  string
	calls int
}

func (p *r7SttProvider) StreamRecognize(context.Context, <-chan []byte) (<-chan stt.TranscriptionResult, error) {
	panic("StreamRecognize is not used in r7 integration tests")
}

func (p *r7SttProvider) RecognizeFile(context.Context, string) (stt.TranscriptionResult, error) {
	p.calls++
	return stt.TranscriptionResult{Text: p.text, IsFinal: true, Confidence: 0.95}, nil
}

func (p *r7SttProvider) Close() error { return nil }

// r7SettingsSource returns a fixed AI settings row for capture creation.
type r7SettingsSource struct {
	settings *entity.UserAISettings
}

func (s *r7SettingsSource) Get(context.Context, uuid.UUID) (*entity.UserAISettings, error) {
	return s.settings, nil
}

// TestR7TraceabilityChainIntegration proves G5 criterion 4 with real
// PostgreSQL: through the full capture -> audio upload (sha256) -> STT
// transcript -> user transcript correction -> user card correction chain, the
// original audio, the original capture text, and the user-corrected card
// version must all remain traceable. Nothing is ever overwritten or deleted.
func TestR7TraceabilityChainIntegration(t *testing.T) {
	databaseURL := os.Getenv("WEAVEBRAIN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WEAVEBRAIN_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	store := repository.NewFromPool(pool)
	userID := uuid.New()
	if _, err := pool.Exec(
		ctx,
		"INSERT INTO users (id, display_name) VALUES ($1, 'traceability-test')",
		userID,
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	captureSvc := NewCaptureService(store.Capture, &r7SettingsSource{
		settings: &entity.UserAISettings{UserID: userID, AIMemoryEnabled: true},
	})

	// Step 1: create the audio capture (fallback card + fallback revision).
	captureID := uuid.New()
	if _, err := captureSvc.Create(ctx, userID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindAudio,
		Source:        "mobile_android",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create audio capture: %v", err)
	}

	// Step 2: chunked audio upload with full-file sha256 verification.
	audioStore, err := NewLocalAudioFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("init audio store: %v", err)
	}
	audioSvc := NewAudioService(
		store.AudioAsset,
		store.Transcript,
		store.Capture,
		audioStore,
		&r7SttProvider{text: "跑步时想到的一个好主意"},
	)

	assetID := uuid.New()
	audioData := []byte("traceable audio payload for G5 criterion 4")
	chunks := splitChunks(audioData, 9)
	if _, err := audioSvc.Initiate(ctx, userID, InitiateAudioAssetInput{
		AssetID:     assetID,
		CaptureID:   captureID,
		MimeType:    "audio/wav",
		SizeBytes:   int64(len(audioData)),
		SHA256:      hashBytes(audioData),
		TotalChunks: int32(len(chunks)),
		STTEnabled:  true,
	}); err != nil {
		t.Fatalf("initiate audio upload: %v", err)
	}
	uploadAllChunks(t, audioSvc, userID, assetID, chunks)
	completed, err := audioSvc.Complete(ctx, userID, assetID)
	if err != nil {
		t.Fatalf("complete audio upload: %v", err)
	}
	if completed.UploadState != entity.AudioUploadStateComplete {
		t.Fatalf("expected upload complete, got %s", completed.UploadState)
	}

	// Step 3: STT transcription appends an immutable stt transcript revision.
	sttRev, err := audioSvc.TranscribeCapture(ctx, userID, captureID)
	if err != nil {
		t.Fatalf("transcribe: %v", err)
	}
	if sttRev.Source != entity.TranscriptSourceSTT || sttRev.Revision != 1 {
		t.Fatalf("expected stt transcript revision 1, got source=%s rev=%d", sttRev.Source, sttRev.Revision)
	}

	// Step 4: user transcript correction appends a user transcript revision.
	userTranscript, err := audioSvc.CorrectTranscript(ctx, userID, captureID, "用户修正后的转写")
	if err != nil {
		t.Fatalf("correct transcript: %v", err)
	}
	if userTranscript.Source != entity.TranscriptSourceUser || userTranscript.Revision != 2 {
		t.Fatalf("expected user transcript revision 2, got source=%s rev=%d", userTranscript.Source, userTranscript.Revision)
	}

	// Step 5: user card correction appends a user enrichment revision and bumps
	// the card version.
	memorySvc := NewMemoryService(store.Memory, store.Capture, store.AudioAsset, store.Transcript)
	newTitle := "用户修正后的记忆标题"
	_, card, err := memorySvc.Correct(ctx, userID, captureID, CorrectMemoryInput{Title: &newTitle})
	if err != nil {
		t.Fatalf("correct memory card: %v", err)
	}
	if card.MemoryCard.Version != 2 {
		t.Fatalf("expected card version 2 after user correction, got %d", card.MemoryCard.Version)
	}

	// Assertion A: original audio still present with unchanged sha256.
	asset, err := audioSvc.GetByID(ctx, userID, assetID)
	if err != nil {
		t.Fatalf("audio asset must remain readable: %v", err)
	}
	if asset.UploadState != entity.AudioUploadStateComplete {
		t.Fatalf("audio asset must remain complete, got %s", asset.UploadState)
	}
	if asset.SHA256 == nil || *asset.SHA256 != hashBytes(audioData) {
		t.Fatalf("audio sha256 must be unchanged after transcribe+correct")
	}

	// Assertion B: original capture text (NULL for an audio capture) must never
	// be rewritten by transcription or correction.
	var originalText *string
	if err := pool.QueryRow(
		ctx,
		"SELECT original_text FROM captures WHERE user_id = $1 AND id = $2",
		userID, captureID,
	).Scan(&originalText); err != nil {
		t.Fatalf("read original text: %v", err)
	}
	if originalText != nil {
		t.Fatalf("audio capture original_text must stay NULL, got %q", *originalText)
	}

	// Assertion C: transcript trail must be [stt, user].
	transcripts, err := store.Transcript.ListByCapture(ctx, userID, captureID)
	if err != nil {
		t.Fatalf("list transcript revisions: %v", err)
	}
	wantTranscriptSources := map[entity.TranscriptSource]int{
		entity.TranscriptSourceSTT:  1,
		entity.TranscriptSourceUser: 1,
	}
	gotTranscriptSources := map[entity.TranscriptSource]int{}
	for _, rev := range transcripts {
		gotTranscriptSources[rev.Source]++
	}
	for source, want := range wantTranscriptSources {
		if gotTranscriptSources[source] != want {
			t.Fatalf("transcript source %s count=%d, want %d", source, gotTranscriptSources[source], want)
		}
	}

	// Assertion D: card revision trail must be [fallback, user].
	revisions, err := store.Memory.ListRevisions(ctx, userID, captureID)
	if err != nil {
		t.Fatalf("list enrichment revisions: %v", err)
	}
	wantCardSources := map[entity.EnrichmentSource]int{
		entity.EnrichmentSourceFallback: 1,
		entity.EnrichmentSourceUser:     1,
	}
	gotCardSources := map[entity.EnrichmentSource]int{}
	for _, rev := range revisions {
		gotCardSources[rev.Source]++
	}
	for source, want := range wantCardSources {
		if gotCardSources[source] != want {
			t.Fatalf("card revision source %s count=%d, want %d", source, gotCardSources[source], want)
		}
	}
}
