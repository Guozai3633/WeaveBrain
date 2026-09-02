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
	ErrDuplicateExternalID        = errors.New("external_id already imported")
)

// DuplicateExternalIDError carries the existing capture id so the API can return
// it to the client. errors.Is(err, ErrDuplicateExternalID) matches the sentinel.
type DuplicateExternalIDError struct {
	ExistingCaptureID uuid.UUID
}

func (e *DuplicateExternalIDError) Error() string {
	return fmt.Sprintf("%v: %s", ErrDuplicateExternalID, e.ExistingCaptureID)
}

func (e *DuplicateExternalIDError) Unwrap() error { return ErrDuplicateExternalID }

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
	// ExternalID/SourceName are the import dedup key (single import). Both must
	// be set together; exact (source_name, external_id) reuse is rejected.
	ExternalID *string
	SourceName *string
	// TitleOverride/PrimaryTypeOverride/TagsOverride let an importer supply the
	// card's initial values (provenance=import) instead of the fallback title.
	TitleOverride      *string
	PrimaryTypeOverride *string
	TagsOverride       []string
}

type CreateCaptureResult struct {
	Aggregate *entity.CaptureAggregate
	Replayed  bool
	// Dedupe is non-nil for a single import whose content hash matches an
	// existing capture: the import still proceeds, the client decides.
	Dedupe *entity.CaptureDedupe
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
		ExternalID:          normalized.ExternalID,
		SourceName:          normalized.SourceName,
		RequestHash:         requestHash,
		ClientVersion:       normalized.ClientVersion,
	}

	// Single-import dedup: exact (source_name, external_id) is a hard reject
	// (excluding the same capture on an idempotent retry); a content-hash match
	// is a soft "suspected duplicate" the client decides about.
	if normalized.Kind == entity.CaptureKindImport &&
		normalized.ExternalID != nil && *normalized.ExternalID != "" &&
		normalized.SourceName != nil && *normalized.SourceName != "" {
		existingID, err := s.repository.FindExternalDuplicate(
			ctx, userID, *normalized.SourceName, *normalized.ExternalID, normalized.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("check external duplicate: %w", err)
		}
		if existingID != nil {
			return nil, &DuplicateExternalIDError{ExistingCaptureID: *existingID}
		}
	}
	var dedupe *entity.CaptureDedupe
	if normalized.Kind == entity.CaptureKindImport {
		hash := normalizeAndHashContent(normalized.Text)
		capture.ContentHash = &hash
		if hash != "" {
			existingID, err := s.repository.FindContentHashMatch(ctx, userID, hash, normalized.ID)
			if err != nil {
				return nil, fmt.Errorf("check content hash duplicate: %w", err)
			}
			if existingID != nil {
				dedupe = &entity.CaptureDedupe{
					Status:            "suggested",
					ExistingCaptureID: existingID,
				}
			}
		}
	}

	title := fallbackTitle(normalized.Text)
	if title == "" {
		title = "语音记录"
	}
	if normalized.TitleOverride != nil && strings.TrimSpace(*normalized.TitleOverride) != "" {
		title = strings.TrimSpace(*normalized.TitleOverride)
	}
	primaryType := "uncategorized"
	if normalized.PrimaryTypeOverride != nil && strings.TrimSpace(*normalized.PrimaryTypeOverride) != "" {
		primaryType = *normalized.PrimaryTypeOverride
	}
	tags := []string{}
	if normalized.TagsOverride != nil {
		tags = normalized.TagsOverride
	}
	card := &entity.MemoryCard{
		ID:               uuid.New(),
		UserID:           userID,
		CaptureID:        normalized.ID,
		PrimaryType:      primaryType,
		Title:            title,
		Summary:          fallbackSummary(normalized.Text),
		Tags:             tags,
		KeyPoints:        []string{},
		ProcessingStatus: "ready",
		Version:          1,
	}
	revision := initialEnrichmentRevision(userID, normalized.ID, title, card.Summary, revisionSourceForKind(normalized.Kind))

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

	return &CreateCaptureResult{Aggregate: aggregate, Replayed: replayed, Dedupe: dedupe}, nil
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
	case entity.CaptureKindImport:
		// Single import reuses the capture pipeline: content is required, just
		// like a text capture. external_id/source_name form the dedup key.
		if strings.TrimSpace(input.Text) == "" {
			return input, fmt.Errorf("%w: import text must not be empty", ErrInvalidCapture)
		}
		if !utf8.ValidString(input.Text) {
			return input, fmt.Errorf("%w: import text must be valid UTF-8", ErrInvalidCapture)
		}
		if utf8.RuneCountInString(input.Text) > MaxCaptureTextRunes {
			return input, fmt.Errorf("%w: import text exceeds %d characters", ErrInvalidCapture, MaxCaptureTextRunes)
		}
		if input.ExternalID != nil {
			if utf8.RuneCountInString(*input.ExternalID) > 200 {
				return input, fmt.Errorf("%w: external_id exceeds 200 characters", ErrInvalidCapture)
			}
			input.ExternalID = trimStringPtr(input.ExternalID)
		}
		if input.SourceName != nil {
			if utf8.RuneCountInString(*input.SourceName) > 100 {
				return input, fmt.Errorf("%w: source_name exceeds 100 characters", ErrInvalidCapture)
			}
			input.SourceName = trimStringPtr(input.SourceName)
		}
	default:
		return input, fmt.Errorf("%w: only text, audio and import captures are supported", ErrInvalidCapture)
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
		if input.Kind == entity.CaptureKindImport {
			input.Source = "import"
		} else {
			input.Source = "api"
		}
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
		ExternalID          *string            `json:"external_id"`
		SourceName          *string            `json:"source_name"`
	}

	payload, err := json.Marshal(canonicalCaptureRequest{
		ID:                  input.ID,
		Kind:                input.Kind,
		Text:                input.Text,
		CapturedAt:          input.CapturedAt,
		CapturedAtPrecision: input.CapturedAtPrecision,
		Timezone:            input.Timezone,
		Source:              input.Source,
		CollectionID:        input.CollectionID,
		PrivacyMode:         input.PrivacyMode,
		ClientVersion:       input.ClientVersion,
		ExternalID:          input.ExternalID,
		SourceName:          input.SourceName,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// normalizeAndHashContent collapses whitespace in the text and returns its
// SHA-256 hex digest. Used as the content-level "suspected duplicate" key.
func normalizeAndHashContent(text string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

// trimStringPtr trims surrounding whitespace and returns nil when empty.
func trimStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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

// initialEnrichmentRevision is the initial revision recorded atomically at
// capture creation. It documents that the card's AI-claimed fields were derived
// locally (source=fallback for text/audio, source=import for imports) from the
// raw capture, before any AI pipeline exists.
func initialEnrichmentRevision(userID uuid.UUID, captureID uuid.UUID, title string, summary *string, source entity.EnrichmentSource) *entity.EnrichmentRevision {
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
		Source:         source,
		SourceRevision: 1,
		Changes:        changes,
		Provenance:     map[string]any{"source": string(source)},
	}
}

// revisionSourceForKind picks the initial revision source for a capture kind.
func revisionSourceForKind(kind entity.CaptureKind) entity.EnrichmentSource {
	if kind == entity.CaptureKindImport {
		return entity.EnrichmentSourceImport
	}
	return entity.EnrichmentSourceFallback
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
