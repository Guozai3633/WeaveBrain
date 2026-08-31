package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

var (
	ErrInvalidCapture             = errors.New("invalid capture")
	ErrCaptureNotFound            = errors.New("capture not found")
	ErrCaptureIdempotencyConflict = errors.New("capture idempotency conflict")
)

const (
	fallbackTitleMaxRunes   = 30
	fallbackSummaryMaxRunes = 200
	MaxCaptureTextRunes     = 100_000
)

type CreateCaptureInput struct {
	ID                  uuid.UUID
	Kind                entity.CaptureKind
	Text                string
	CapturedAt          *time.Time
	CapturedAtPrecision string
	Timezone            *string
	Source              string
	CollectionID        *int64
	PrivacyMode         string
	ClientVersion       int
}

type CreateCaptureResult struct {
	Aggregate *entity.CaptureAggregate
	Replayed  bool
}

// AISettingsSnapshotSource resolves the user's current AI consent switches so
// a newly created Capture can carry them as a policy snapshot.
type AISettingsSnapshotSource interface {
	Get(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error)
}

type CaptureService struct {
	repository repository.CaptureRepository
	settings   AISettingsSnapshotSource
}

// NewCaptureService creates a CaptureService. The settings source may be nil;
// when it is nil (or fails to read), captures fall back to the privacy-first
// all-disabled snapshot so a settings outage never blocks capture creation.
func NewCaptureService(repo repository.CaptureRepository, settings AISettingsSnapshotSource) *CaptureService {
	return &CaptureService{repository: repo, settings: settings}
}

func (s *CaptureService) Create(
	ctx context.Context,
	userID uuid.UUID,
	input CreateCaptureInput,
) (*CreateCaptureResult, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("capture repository is not configured")
	}

	normalized, err := normalizeCreateCaptureInput(userID, input)
	if err != nil {
		return nil, err
	}

	requestHash, err := hashCaptureRequest(normalized)
	if err != nil {
		return nil, fmt.Errorf("hash capture request: %w", err)
	}

	var originalText *string
	if trimmed := strings.TrimSpace(normalized.Text); trimmed != "" {
		originalText = &normalized.Text
	}
	capture := &entity.Capture{
		ID:                  normalized.ID,
		UserID:              userID,
		Kind:                normalized.Kind,
		OriginalText:        originalText,
		CapturedAt:          normalized.CapturedAt,
		CapturedAtPrecision: normalized.CapturedAtPrecision,
		Timezone:            normalized.Timezone,
		Source:              normalized.Source,
		CollectionID:        normalized.CollectionID,
		PrivacyMode:         normalized.PrivacyMode,
		RequestHash:         requestHash,
		ClientVersion:       normalized.ClientVersion,
	}
	title := fallbackTitle(normalized.Text)
	if title == "" {
		title = "语音记录"
	}
	card := &entity.MemoryCard{
		ID:               uuid.New(),
		UserID:           userID,
		CaptureID:        normalized.ID,
		PrimaryType:      "uncategorized",
		Title:            title,
		Summary:          fallbackSummary(normalized.Text),
		Tags:             []string{},
		KeyPoints:        []string{},
		ProcessingStatus: "ready",
		Version:          1,
	}
	revision := fallbackEnrichmentRevision(userID, normalized.ID, title, card.Summary)

	replayed, err := s.repository.Create(ctx, capture, card, revision, s.resolvePolicySnapshot(ctx, userID))
	if errors.Is(err, repository.ErrCaptureIdempotencyConflict) {
		return nil, ErrCaptureIdempotencyConflict
	}
	if err != nil {
		return nil, fmt.Errorf("create capture: %w", err)
	}

	aggregate, err := s.repository.GetByID(ctx, userID, normalized.ID)
	if err != nil {
		return nil, mapCaptureRepositoryError("load created capture", err)
	}

	return &CreateCaptureResult{Aggregate: aggregate, Replayed: replayed}, nil
}

// resolvePolicySnapshot reads the user's AI consent switches at capture time.
// A missing source or a settings read failure falls back to the privacy-first
// all-disabled snapshot; capture creation must never be blocked by settings.
func (s *CaptureService) resolvePolicySnapshot(ctx context.Context, userID uuid.UUID) *entity.PolicySnapshot {
	if s.settings == nil {
		return entity.DefaultPolicySnapshot()
	}
	settings, err := s.settings.Get(ctx, userID)
	if err != nil {
		return entity.DefaultPolicySnapshot()
	}
	return entity.FromAISettings(settings)
}

func (s *CaptureService) GetByID(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CaptureAggregate, error) {
	if s == nil || s.repository == nil {
		return nil, fmt.Errorf("capture repository is not configured")
	}
	if userID == uuid.Nil || captureID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id and capture_id are required", ErrInvalidCapture)
	}

	aggregate, err := s.repository.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, mapCaptureRepositoryError("get capture", err)
	}
	return aggregate, nil
}

func normalizeCreateCaptureInput(
	userID uuid.UUID,
	input CreateCaptureInput,
) (CreateCaptureInput, error) {
	if userID == uuid.Nil {
		return input, fmt.Errorf("%w: user_id is required", ErrInvalidCapture)
	}
	if input.ID == uuid.Nil {
		return input, fmt.Errorf("%w: capture_id is required", ErrInvalidCapture)
	}
	if input.Kind == "" {
		input.Kind = entity.CaptureKindText
	}
	switch input.Kind {
	case entity.CaptureKindText:
		if strings.TrimSpace(input.Text) == "" {
			return input, fmt.Errorf("%w: text must not be empty", ErrInvalidCapture)
		}
		if !utf8.ValidString(input.Text) {
			return input, fmt.Errorf("%w: text must be valid UTF-8", ErrInvalidCapture)
		}
		if utf8.RuneCountInString(input.Text) > MaxCaptureTextRunes {
			return input, fmt.Errorf("%w: text exceeds %d characters", ErrInvalidCapture, MaxCaptureTextRunes)
		}
	case entity.CaptureKindAudio:
		// Audio captures carry no text at creation time; the audio file and an
		// optional STT/user transcript provide content later. Empty text is valid.
		if !utf8.ValidString(input.Text) {
			return input, fmt.Errorf("%w: text must be valid UTF-8", ErrInvalidCapture)
		}
	default:
		return input, fmt.Errorf("%w: only text and audio captures are supported", ErrInvalidCapture)
	}

	if input.CapturedAtPrecision == "" {
		input.CapturedAtPrecision = "unknown"
	}
	switch input.CapturedAtPrecision {
	case "exact", "date_only", "estimated", "unknown":
	default:
		return input, fmt.Errorf("%w: invalid captured_at_precision", ErrInvalidCapture)
	}
	if input.Source == "" {
		input.Source = "api"
	}
	if input.PrivacyMode == "" {
		input.PrivacyMode = "cloud_allowed"
	}
	if input.PrivacyMode != "cloud_allowed" && input.PrivacyMode != "no_ai" {
		return input, fmt.Errorf("%w: invalid privacy_mode", ErrInvalidCapture)
	}
	if input.ClientVersion <= 0 {
		return input, fmt.Errorf("%w: client_version must be greater than zero", ErrInvalidCapture)
	}

	input.Source = strings.TrimSpace(input.Source)
	if input.Source == "" {
		return input, fmt.Errorf("%w: source must not be empty", ErrInvalidCapture)
	}
	if input.Timezone != nil {
		trimmed := strings.TrimSpace(*input.Timezone)
		if trimmed == "" {
			input.Timezone = nil
		} else {
			input.Timezone = &trimmed
		}
	}
	return input, nil
}

func hashCaptureRequest(input CreateCaptureInput) (string, error) {
	type canonicalCaptureRequest struct {
		ID                  uuid.UUID          `json:"capture_id"`
		Kind                entity.CaptureKind `json:"kind"`
		Text                string             `json:"text"`
		CapturedAt          *time.Time         `json:"captured_at"`
		CapturedAtPrecision string             `json:"captured_at_precision"`
		Timezone            *string            `json:"timezone"`
		Source              string             `json:"source"`
		CollectionID        *int64             `json:"collection_id"`
		PrivacyMode         string             `json:"privacy_mode"`
		ClientVersion       int                `json:"client_version"`
	}

	payload, err := json.Marshal(canonicalCaptureRequest(input))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func fallbackTitle(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) > fallbackTitleMaxRunes {
		runes = runes[:fallbackTitleMaxRunes]
	}
	return string(runes)
}

// fallbackSummary derives a local summary for the fallback card: the first
// 200 runes plus an ellipsis when the text is longer. It returns nil for
// captures with no text (e.g. audio captures before a transcript exists).
func fallbackSummary(text string) *string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	runes := []rune(trimmed)
	if len(runes) > fallbackSummaryMaxRunes {
		runes = runes[:fallbackSummaryMaxRunes]
	}
	summary := string(runes)
	if utf8.RuneCountInString(trimmed) > fallbackSummaryMaxRunes {
		summary += "…"
	}
	return &summary
}

// fallbackEnrichmentRevision is the initial revision recorded atomically at
// capture creation. It documents that the card's AI-claimed fields were derived
// locally (source=fallback) from the raw capture, before any AI pipeline exists.
func fallbackEnrichmentRevision(userID uuid.UUID, captureID uuid.UUID, title string, summary *string) *entity.EnrichmentRevision {
	changes := map[string]any{
		"title":        title,
		"primary_type": "uncategorized",
		"tags":         []string{},
		"key_points":   []string{},
	}
	if summary != nil {
		changes["summary"] = *summary
	}
	return &entity.EnrichmentRevision{
		UserID:         userID,
		CaptureID:      captureID,
		CardVersion:    1,
		Source:         entity.EnrichmentSourceFallback,
		SourceRevision: 1,
		Changes:        changes,
		Provenance:     map[string]any{"source": "fallback"},
	}
}

func mapCaptureRepositoryError(operation string, err error) error {
	if errors.Is(err, repository.ErrCaptureNotFound) {
		return ErrCaptureNotFound
	}
	if errors.Is(err, repository.ErrCaptureIdempotencyConflict) {
		return ErrCaptureIdempotencyConflict
	}
	return fmt.Errorf("%s: %w", operation, err)
}
