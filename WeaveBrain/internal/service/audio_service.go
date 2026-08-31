package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"
	"weavebrain/internal/stt"

	"github.com/google/uuid"
)

var (
	ErrInvalidAudioAsset     = errors.New("invalid audio asset")
	ErrAudioAssetNotFound    = errors.New("audio asset not found")
	ErrAudioUploadNotReady   = errors.New("audio upload not complete")
	ErrAudioChecksumMismatch = errors.New("audio checksum mismatch")
	ErrAudioChunkOutOfRange  = errors.New("audio chunk index out of range")
	ErrAudioChunkTooLarge    = errors.New("audio chunk exceeds maximum size")
	ErrInvalidTranscript     = errors.New("invalid transcript")
)

const (
	MaxAudioChunkBytes = 5 << 20 // 5 MiB per chunk
	MaxAudioFileBytes  = 200 << 20 // 200 MiB per audio file
	MaxTranscriptRunes = 100_000
)

// InitiateAudioAssetInput is the validated input to begin a chunked upload.
type InitiateAudioAssetInput struct {
	AssetID    uuid.UUID
	CaptureID  uuid.UUID
	MimeType   string
	DurationMs *int64
	SizeBytes  int64
	SHA256     string // full-file SHA-256, lowercase hex
	TotalChunks int32
	STTEnabled bool
}

// AudioService orchestrates chunked audio upload, checksum verification,
// transcription and user transcript correction.
type AudioService struct {
	assets      repository.AudioAssetRepository
	transcripts repository.TranscriptRepository
	captures    repository.CaptureRepository
	files       AudioFileStore
	sttProvider stt.STTProvider
}

func NewAudioService(
	assets repository.AudioAssetRepository,
	transcripts repository.TranscriptRepository,
	captures repository.CaptureRepository,
	files AudioFileStore,
	sttProvider stt.STTProvider,
) *AudioService {
	return &AudioService{
		assets:      assets,
		transcripts: transcripts,
		captures:    captures,
		files:       files,
		sttProvider: sttProvider,
	}
}

// Initiate begins a new chunked upload for a Capture. It is idempotent: calling
// it again with the same asset ID returns the existing asset.
func (s *AudioService) Initiate(
	ctx context.Context,
	userID uuid.UUID,
	input InitiateAudioAssetInput,
) (*entity.AudioAsset, error) {
	if s == nil || s.assets == nil {
		return nil, fmt.Errorf("audio asset repository is not configured")
	}
	if err := validateInitiateAudioInput(userID, input); err != nil {
		return nil, err
	}

	capture, err := s.captures.GetByID(ctx, userID, input.CaptureID)
	if err != nil {
		if errors.Is(err, repository.ErrCaptureNotFound) {
			return nil, fmt.Errorf("%w: capture not found", ErrInvalidAudioAsset)
		}
		return nil, fmt.Errorf("load capture for audio asset: %w", err)
	}
	if capture.Capture.Kind != entity.CaptureKindAudio {
		return nil, fmt.Errorf("%w: capture kind must be audio", ErrInvalidAudioAsset)
	}

	extension := extensionForMime(input.MimeType)
	asset := &entity.AudioAsset{
		ID:          input.AssetID,
		UserID:      userID,
		CaptureID:   input.CaptureID,
		MimeType:    input.MimeType,
		DurationMs:  input.DurationMs,
		SizeBytes:   input.SizeBytes,
		SHA256:      &input.SHA256,
		StoragePath: storagePathFor(userID, input.AssetID, extension),
		UploadState: entity.AudioUploadStateInitiated,
		TotalChunks: input.TotalChunks,
		STTEnabled:  input.STTEnabled,
	}
	if err := s.assets.Initiate(ctx, asset); err != nil {
		return nil, fmt.Errorf("initiate audio asset: %w", err)
	}

	stored, err := s.assets.GetByID(ctx, userID, input.AssetID)
	if err != nil {
		return nil, fmt.Errorf("load audio asset after initiate: %w", err)
	}
	if stored.CaptureID != input.CaptureID {
		return nil, fmt.Errorf("%w: asset already bound to a different capture", ErrInvalidAudioAsset)
	}
	return stored, nil
}

// UploadChunk persists a single chunk after verifying its SHA-256.
func (s *AudioService) UploadChunk(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
	chunkIndex int32,
	declaredHash string,
	data []byte,
) error {
	asset, err := s.assets.GetByID(ctx, userID, assetID)
	if err != nil {
		return mapAudioRepositoryError(err)
	}
	if asset.UploadState == entity.AudioUploadStateComplete {
		return nil // already complete; re-sending chunks is a no-op
	}
	if chunkIndex < 0 || chunkIndex >= asset.TotalChunks {
		return fmt.Errorf("%w: index=%d total=%d", ErrAudioChunkOutOfRange, chunkIndex, asset.TotalChunks)
	}
	if len(data) > MaxAudioChunkBytes {
		return fmt.Errorf("%w: %d bytes", ErrAudioChunkTooLarge, len(data))
	}
	if asset.SizeBytes > MaxAudioFileBytes {
		return fmt.Errorf("%w: file exceeds %d bytes", ErrInvalidAudioAsset, MaxAudioFileBytes)
	}

	hash := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(hash[:]), declaredHash) {
		return ErrAudioChecksumMismatch
	}

	if err := s.files.WriteChunk(ctx, assetID, chunkIndex, data); err != nil {
		return fmt.Errorf("write chunk: %w", err)
	}

	present, err := s.files.ListStagedChunks(assetID)
	if err != nil {
		return err
	}
	if err := s.assets.UpdateChunkProgress(ctx, userID, assetID, int32(len(present))); err != nil {
		return mapAudioRepositoryError(err)
	}
	return nil
}

// Complete verifies all chunks are present, concatenates them, verifies the
// full-file SHA-256 and marks the upload complete.
func (s *AudioService) Complete(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
) (*entity.AudioAsset, error) {
	asset, err := s.assets.GetByID(ctx, userID, assetID)
	if err != nil {
		return nil, mapAudioRepositoryError(err)
	}
	if asset.UploadState == entity.AudioUploadStateComplete {
		return asset, nil // idempotent completion
	}

	present, err := s.files.ListStagedChunks(assetID)
	if err != nil {
		return nil, err
	}
	if int32(len(present)) < asset.TotalChunks {
		return nil, fmt.Errorf(
			"%w: received %d/%d chunks",
			ErrAudioUploadNotReady,
			len(present),
			asset.TotalChunks,
		)
	}
	if len(present) > int(asset.TotalChunks) {
		return nil, fmt.Errorf("%w: received more chunks than declared", ErrInvalidAudioAsset)
	}
	indices := stagedChunkIndices(present)
	for i, idx := range indices {
		if int32(i) != idx {
			return nil, fmt.Errorf("%w: missing chunk %d", ErrAudioUploadNotReady, i)
		}
	}

	finalPath, size, err := s.files.Finalize(ctx, assetID, asset.StoragePath, asset.TotalChunks)
	if err != nil {
		return nil, err
	}
	if size != asset.SizeBytes {
		s.files.RemoveStaging(ctx, assetID)
		return nil, fmt.Errorf("%w: size=%d declared=%d", ErrAudioChecksumMismatch, size, asset.SizeBytes)
	}

	fullHash, err := hashFile(finalPath)
	if err != nil {
		return nil, fmt.Errorf("hash final audio: %w", err)
	}
	if !strings.EqualFold(fullHash, *asset.SHA256) {
		s.files.RemoveStaging(ctx, assetID)
		return nil, fmt.Errorf("%w: full-file sha256 mismatch", ErrAudioChecksumMismatch)
	}

	if err := s.assets.Complete(ctx, userID, assetID, fullHash); err != nil {
		return nil, mapAudioRepositoryError(err)
	}
	_ = s.files.RemoveStaging(ctx, assetID)

	return s.assets.GetByID(ctx, userID, assetID)
}

// TranscribeCapture runs STT on a completed audio asset and stores the resulting
// transcript. The original audio is never deleted, even if STT fails.
func (s *AudioService) TranscribeCapture(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.TranscriptRevision, error) {
	asset, err := s.assets.GetByCaptureID(ctx, userID, captureID)
	if err != nil {
		return nil, mapAudioRepositoryError(err)
	}
	if asset.UploadState != entity.AudioUploadStateComplete {
		return nil, fmt.Errorf("%w: audio not uploaded", ErrAudioUploadNotReady)
	}
	if !asset.STTEnabled {
		return nil, fmt.Errorf("%w: stt disabled for this capture", ErrInvalidAudioAsset)
	}
	if s.sttProvider == nil {
		return nil, fmt.Errorf("STT provider is not configured")
	}

	reader, err := s.files.OpenFinal(ctx, asset.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("open final audio: %w", err)
	}
	defer reader.Close()
	// The provider reads from the file path; a warm open verifies accessibility.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return nil, fmt.Errorf("read final audio: %w", err)
	}

	result, err := s.sttProvider.RecognizeFile(ctx, asset.StoragePath)
	if err != nil {
		return nil, fmt.Errorf("stt recognize: %w", err)
	}
	text := strings.TrimSpace(result.Text)
	if text == "" {
		return nil, fmt.Errorf("stt returned empty transcript")
	}

	revision := &entity.TranscriptRevision{
		UserID:     userID,
		CaptureID:  captureID,
		Text:       text,
		Source:     entity.TranscriptSourceSTT,
		Confidence: ptrFloat32(result.Confidence),
	}
	if err := s.transcripts.Append(ctx, revision); err != nil {
		return nil, fmt.Errorf("append transcript: %w", err)
	}
	_ = s.captures.UpdateCardTitle(ctx, userID, captureID, fallbackTitle(text))

	return revision, nil
}

// CorrectTranscript records a user-corrected transcript (highest priority).
func (s *AudioService) CorrectTranscript(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	text string,
) (*entity.TranscriptRevision, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, fmt.Errorf("%w: text must not be empty", ErrInvalidTranscript)
	}
	if !utf8.ValidString(trimmed) {
		return nil, fmt.Errorf("%w: text must be valid UTF-8", ErrInvalidTranscript)
	}
	if utf8.RuneCountInString(trimmed) > MaxTranscriptRunes {
		return nil, fmt.Errorf("%w: text exceeds %d characters", ErrInvalidTranscript, MaxTranscriptRunes)
	}

	revision := &entity.TranscriptRevision{
		UserID:     userID,
		CaptureID:  captureID,
		Text:       trimmed,
		Source:     entity.TranscriptSourceUser,
		Confidence: nil,
	}
	if err := s.transcripts.Append(ctx, revision); err != nil {
		return nil, fmt.Errorf("append user transcript: %w", err)
	}
	_ = s.captures.UpdateCardTitle(ctx, userID, captureID, fallbackTitle(trimmed))

	return revision, nil
}

// GetByID loads a single audio asset scoped to the user.
func (s *AudioService) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	assetID uuid.UUID,
) (*entity.AudioAsset, error) {
	if s == nil || s.assets == nil {
		return nil, fmt.Errorf("audio asset repository is not configured")
	}
	if userID == uuid.Nil || assetID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id and asset_id are required", ErrInvalidAudioAsset)
	}
	asset, err := s.assets.GetByID(ctx, userID, assetID)
	if err != nil {
		return nil, mapAudioRepositoryError(err)
	}
	return asset, nil
}

// GetLatestTranscript returns the newest transcript revision for a Capture.
func (s *AudioService) GetLatestTranscript(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.TranscriptRevision, error) {
	revision, err := s.transcripts.GetLatest(ctx, userID, captureID)
	if err != nil {
		return nil, mapAudioRepositoryError(err)
	}
	return revision, nil
}

// GetByCapture returns a Capture together with its audio asset (if any) and the
// latest transcript revision. Text captures simply have Audio and Transcript nil.
func (s *AudioService) GetByCapture(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CaptureAudioAggregate, error) {
	if s == nil || s.captures == nil {
		return nil, fmt.Errorf("capture repository is not configured")
	}
	aggregate, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		if errors.Is(err, repository.ErrCaptureNotFound) {
			return nil, ErrCaptureNotFound
		}
		return nil, fmt.Errorf("load capture: %w", err)
	}

	agg := &entity.CaptureAudioAggregate{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
	}
	asset, err := s.assets.GetByCaptureID(ctx, userID, captureID)
	if err == nil {
		agg.Audio = asset
		transcript, terr := s.transcripts.GetLatest(ctx, userID, captureID)
		if terr == nil {
			agg.Transcript = transcript
		}
		return agg, nil
	}
	if errors.Is(err, repository.ErrAudioAssetNotFound) {
		// No audio asset attached (e.g. a text capture); not an error.
		return agg, nil
	}
	return nil, mapAudioRepositoryError(err)
}

func validateInitiateAudioInput(userID uuid.UUID, input InitiateAudioAssetInput) error {
	if userID == uuid.Nil {
		return fmt.Errorf("%w: user_id is required", ErrInvalidAudioAsset)
	}
	if input.AssetID == uuid.Nil {
		return fmt.Errorf("%w: asset_id is required", ErrInvalidAudioAsset)
	}
	if input.CaptureID == uuid.Nil {
		return fmt.Errorf("%w: capture_id is required", ErrInvalidAudioAsset)
	}
	if strings.TrimSpace(input.MimeType) == "" {
		return fmt.Errorf("%w: mime_type is required", ErrInvalidAudioAsset)
	}
	if input.SizeBytes <= 0 {
		return fmt.Errorf("%w: size_bytes must be positive", ErrInvalidAudioAsset)
	}
	if input.SizeBytes > MaxAudioFileBytes {
		return fmt.Errorf("%w: file exceeds %d bytes", ErrInvalidAudioAsset, MaxAudioFileBytes)
	}
	if input.TotalChunks <= 0 {
		return fmt.Errorf("%w: total_chunks must be positive", ErrInvalidAudioAsset)
	}
	if strings.TrimSpace(input.SHA256) == "" {
		return fmt.Errorf("%w: sha256 is required", ErrInvalidAudioAsset)
	}
	decoded, err := hex.DecodeString(input.SHA256)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("%w: sha256 must be 64 hex chars", ErrInvalidAudioAsset)
	}
	if input.DurationMs != nil && *input.DurationMs < 0 {
		return fmt.Errorf("%w: duration_ms must not be negative", ErrInvalidAudioAsset)
	}
	return nil
}

func storagePathFor(userID uuid.UUID, assetID uuid.UUID, extension string) string {
	return fmt.Sprintf("audio/%s/%s.%s", userID.String(), assetID.String(), extension)
}

func extensionForMime(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "audio/wav", "audio/wave", "audio/x-wav":
		return "wav"
	case "audio/webm":
		return "webm"
	case "audio/aac", "audio/aacp":
		return "aac"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/ogg", "audio/opus":
		return "ogg"
	case "audio/m4a":
		return "m4a"
	default:
		return "bin"
	}
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func mapAudioRepositoryError(err error) error {
	if errors.Is(err, repository.ErrAudioAssetNotFound) {
		return ErrAudioAssetNotFound
	}
	if errors.Is(err, repository.ErrTranscriptNotFound) {
		return ErrAudioAssetNotFound
	}
	return err
}

func ptrFloat32(v float32) *float32 { return &v }
