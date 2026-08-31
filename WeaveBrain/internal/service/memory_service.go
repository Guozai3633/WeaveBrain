package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

var (
	ErrMemoryNotFound        = errors.New("memory not found")
	ErrInvalidMemoryInput    = errors.New("invalid memory input")
	ErrMemoryVersionConflict = errors.New("memory version conflict")
	ErrInvalidCursor         = errors.New("invalid cursor")
)

const (
	defaultMemoryPageLimit = 20
	maxMemoryPageLimit     = 50

	maxMemoryTitleRunes    = 300
	maxMemorySummaryRunes  = 2000
	maxMemoryTags          = 20
	maxMemoryTagRunes      = 100
	maxMemoryKeyPoints     = 3
	maxMemoryKeyPointRunes = 500
	maxNoteRunes           = 10_000
)

var validMemoryPrimaryTypes = map[string]bool{
	"uncategorized": true,
	"idea":          true,
	"question":      true,
	"action":        true,
	"reflection":    true,
	"reference":     true,
}

// MemoryService exposes the memory stream: list/search, detail with revision
// trail, user correction (修正), continuation notes (续写), pinning and the
// archive/trash lifecycle.
type MemoryService struct {
	memory     repository.MemoryRepository
	captures   repository.CaptureRepository
	audio      repository.AudioAssetRepository
	transcript repository.TranscriptRepository
}

// NewMemoryService creates a MemoryService. audio and transcript may be nil;
// detail views then omit the audio asset and transcript.
func NewMemoryService(
	memory repository.MemoryRepository,
	captures repository.CaptureRepository,
	audio repository.AudioAssetRepository,
	transcript repository.TranscriptRepository,
) *MemoryService {
	return &MemoryService{
		memory:     memory,
		captures:   captures,
		audio:      audio,
		transcript: transcript,
	}
}

// List returns one page of the user's memory stream.
func (s *MemoryService) List(
	ctx context.Context,
	userID uuid.UUID,
	q entity.MemoryListQuery,
) (*entity.MemoryListResult, error) {
	if s == nil || s.memory == nil {
		return nil, fmt.Errorf("memory repository is not configured")
	}
	if q.Limit <= 0 {
		q.Limit = defaultMemoryPageLimit
	}
	if q.Limit > maxMemoryPageLimit {
		q.Limit = maxMemoryPageLimit
	}
	if q.LifecycleStatus == "" {
		q.LifecycleStatus = "active"
	}
	if q.LifecycleStatus != "active" && q.LifecycleStatus != "archived" {
		return nil, fmt.Errorf("%w: lifecycle_status must be active or archived", ErrInvalidMemoryInput)
	}

	result, err := s.memory.List(ctx, userID, q)
	if err != nil {
		return nil, mapMemoryRepositoryError("list memory stream", err)
	}
	return result, nil
}

// GetDetail returns a memory's full detail view: capture, card, optional audio
// asset, optional latest transcript, and the card's revision trail.
func (s *MemoryService) GetDetail(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.MemoryAggregate, error) {
	if s == nil || s.captures == nil {
		return nil, fmt.Errorf("capture repository is not configured")
	}
	if userID == uuid.Nil || captureID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id and capture_id are required", ErrInvalidMemoryInput)
	}

	aggregate, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, mapMemoryRepositoryError("load memory detail", err)
	}
	revisions, err := s.memory.ListRevisions(ctx, userID, captureID)
	if err != nil {
		return nil, mapMemoryRepositoryError("load memory revisions", err)
	}

	detail := &entity.MemoryAggregate{
		Capture:    aggregate.Capture,
		MemoryCard: aggregate.MemoryCard,
		Revisions:  revisions,
	}
	if s.audio != nil {
		audio, aerr := s.audio.GetByCaptureID(ctx, userID, captureID)
		if aerr != nil && !errors.Is(aerr, repository.ErrAudioAssetNotFound) {
			return nil, fmt.Errorf("load memory audio: %w", aerr)
		}
		detail.Audio = audio
	}
	if s.transcript != nil {
		transcript, terr := s.transcript.GetLatest(ctx, userID, captureID)
		if terr != nil && !errors.Is(terr, repository.ErrTranscriptNotFound) {
			return nil, fmt.Errorf("load memory transcript: %w", terr)
		}
		detail.Transcript = transcript
	}
	return detail, nil
}

// CorrectMemoryInput carries the fields a user wants to correct. A nil pointer
// / nil slice means the field is left unchanged.
type CorrectMemoryInput struct {
	Title       *string  `json:"title"`
	Summary     *string  `json:"summary"`
	PrimaryType *string  `json:"primary_type"`
	Tags        []string `json:"tags"`
	KeyPoints   []string `json:"key_points"`
}

// Correct applies a user's field corrections, recording a user-source revision.
func (s *MemoryService) Correct(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	input CorrectMemoryInput,
) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
	if s == nil || s.memory == nil || s.captures == nil {
		return nil, nil, fmt.Errorf("memory repository is not configured")
	}
	if userID == uuid.Nil || captureID == uuid.Nil {
		return nil, nil, fmt.Errorf("%w: user_id and capture_id are required", ErrInvalidMemoryInput)
	}

	existing, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, nil, mapMemoryRepositoryError("load memory for correction", err)
	}
	if existing.MemoryCard == nil {
		return nil, nil, ErrMemoryNotFound
	}
	current := existing.MemoryCard

	updated := &entity.MemoryCard{
		ID:               current.ID,
		UserID:           current.UserID,
		CaptureID:        current.CaptureID,
		PrimaryType:      current.PrimaryType,
		Title:            current.Title,
		Summary:          current.Summary,
		Tags:             current.Tags,
		KeyPoints:        current.KeyPoints,
		ProcessingStatus: current.ProcessingStatus,
		Version:          current.Version,
		IsPinned:         current.IsPinned,
		PinnedAt:         current.PinnedAt,
		CreatedAt:        current.CreatedAt,
		UpdatedAt:        current.UpdatedAt,
	}

	changes := map[string]any{}
	provenance := map[string]any{}

	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return nil, nil, fmt.Errorf("%w: title must not be empty", ErrInvalidMemoryInput)
		}
		if utf8.RuneCountInString(title) > maxMemoryTitleRunes {
			return nil, nil, fmt.Errorf("%w: title exceeds %d characters", ErrInvalidMemoryInput, maxMemoryTitleRunes)
		}
		updated.Title = title
		changes["title"] = title
		provenance["title"] = "user"
	}
	if input.Summary != nil {
		summary := strings.TrimSpace(*input.Summary)
		if utf8.RuneCountInString(summary) > maxMemorySummaryRunes {
			return nil, nil, fmt.Errorf("%w: summary exceeds %d characters", ErrInvalidMemoryInput, maxMemorySummaryRunes)
		}
		if summary == "" {
			updated.Summary = nil
			changes["summary"] = nil
		} else {
			updated.Summary = &summary
			changes["summary"] = summary
		}
		provenance["summary"] = "user"
	}
	if input.PrimaryType != nil {
		primaryType := *input.PrimaryType
		if !validMemoryPrimaryTypes[primaryType] {
			return nil, nil, fmt.Errorf("%w: invalid primary_type", ErrInvalidMemoryInput)
		}
		updated.PrimaryType = primaryType
		changes["primary_type"] = primaryType
		provenance["primary_type"] = "user"
	}
	if input.Tags != nil {
		if len(input.Tags) > maxMemoryTags {
			return nil, nil, fmt.Errorf("%w: tags exceed %d items", ErrInvalidMemoryInput, maxMemoryTags)
		}
		tags := make([]string, 0, len(input.Tags))
		for _, tag := range input.Tags {
			tag = strings.TrimSpace(tag)
			if utf8.RuneCountInString(tag) > maxMemoryTagRunes {
				return nil, nil, fmt.Errorf("%w: a tag exceeds %d characters", ErrInvalidMemoryInput, maxMemoryTagRunes)
			}
			tags = append(tags, tag)
		}
		updated.Tags = tags
		changes["tags"] = tags
		provenance["tags"] = "user"
	}
	if input.KeyPoints != nil {
		if len(input.KeyPoints) > maxMemoryKeyPoints {
			return nil, nil, fmt.Errorf("%w: key_points exceed %d items", ErrInvalidMemoryInput, maxMemoryKeyPoints)
		}
		keyPoints := make([]string, 0, len(input.KeyPoints))
		for _, kp := range input.KeyPoints {
			kp = strings.TrimSpace(kp)
			if utf8.RuneCountInString(kp) > maxMemoryKeyPointRunes {
				return nil, nil, fmt.Errorf("%w: a key point exceeds %d characters", ErrInvalidMemoryInput, maxMemoryKeyPointRunes)
			}
			keyPoints = append(keyPoints, kp)
		}
		updated.KeyPoints = keyPoints
		changes["key_points"] = keyPoints
		provenance["key_points"] = "user"
	}

	if len(changes) == 0 {
		return nil, nil, fmt.Errorf("%w: nothing to correct", ErrInvalidMemoryInput)
	}

	rev := &entity.EnrichmentRevision{
		UserID:         userID,
		CaptureID:      captureID,
		Source:         entity.EnrichmentSourceUser,
		SourceRevision: existing.Capture.Version,
		Changes:        changes,
		Provenance:     provenance,
	}

	revision, card, err := s.memory.AppendRevisionAndUpdateCard(ctx, userID, captureID, rev, updated)
	if err != nil {
		return nil, nil, mapMemoryRepositoryError("apply correction", err)
	}
	return revision, &entity.CaptureAggregate{Capture: existing.Capture, MemoryCard: card}, nil
}

// AppendNote appends a continuation note (续写) to a memory as a user-source
// revision without rewriting the original text.
func (s *MemoryService) AppendNote(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	text string,
) (*entity.EnrichmentRevision, *entity.CaptureAggregate, error) {
	if s == nil || s.memory == nil || s.captures == nil {
		return nil, nil, fmt.Errorf("memory repository is not configured")
	}
	note := strings.TrimSpace(text)
	if note == "" {
		return nil, nil, fmt.Errorf("%w: note text must not be empty", ErrInvalidMemoryInput)
	}
	if utf8.RuneCountInString(note) > maxNoteRunes {
		return nil, nil, fmt.Errorf("%w: note exceeds %d characters", ErrInvalidMemoryInput, maxNoteRunes)
	}

	existing, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, nil, mapMemoryRepositoryError("load memory for note", err)
	}
	rev := &entity.EnrichmentRevision{
		UserID:         userID,
		CaptureID:      captureID,
		Source:         entity.EnrichmentSourceUser,
		SourceRevision: existing.Capture.Version,
		Changes:        map[string]any{"note": note},
		Provenance:     map[string]any{"note": "user"},
	}

	revision, card, err := s.memory.AppendNoteAndBump(ctx, userID, captureID, rev)
	if err != nil {
		return nil, nil, mapMemoryRepositoryError("append note", err)
	}
	return revision, &entity.CaptureAggregate{Capture: existing.Capture, MemoryCard: card}, nil
}

// SetPinned toggles the pinned flag on a memory card.
func (s *MemoryService) SetPinned(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	pinned bool,
) (*entity.CaptureAggregate, error) {
	if s == nil || s.memory == nil || s.captures == nil {
		return nil, fmt.Errorf("memory repository is not configured")
	}
	aggregate, err := s.captures.GetByID(ctx, userID, captureID)
	if err != nil {
		return nil, mapMemoryRepositoryError("load memory for pin", err)
	}
	card, err := s.memory.SetPinned(ctx, userID, captureID, pinned)
	if err != nil {
		return nil, mapMemoryRepositoryError("set pinned", err)
	}
	aggregate.MemoryCard = card
	return aggregate, nil
}

// Archive moves a memory to the archived lifecycle (idempotent).
func (s *MemoryService) Archive(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CaptureAggregate, error) {
	if s == nil || s.memory == nil {
		return nil, fmt.Errorf("memory repository is not configured")
	}
	aggregate, err := s.memory.SetLifecycle(ctx, userID, captureID, "archived")
	if err != nil {
		return nil, mapMemoryRepositoryError("archive memory", err)
	}
	return aggregate, nil
}

// Delete soft-deletes a memory by moving it to the trashed lifecycle. A memory
// that is already trashed/deleted reports not found.
func (s *MemoryService) Delete(
	ctx context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CaptureAggregate, error) {
	if s == nil || s.memory == nil {
		return nil, fmt.Errorf("memory repository is not configured")
	}
	aggregate, err := s.memory.SetLifecycle(ctx, userID, captureID, "trashed")
	if err != nil {
		return nil, mapMemoryRepositoryError("delete memory", err)
	}
	return aggregate, nil
}

func mapMemoryRepositoryError(operation string, err error) error {
	if errors.Is(err, repository.ErrMemoryNotFound) || errors.Is(err, repository.ErrCaptureNotFound) {
		return ErrMemoryNotFound
	}
	if errors.Is(err, repository.ErrMemoryVersionConflict) {
		return ErrMemoryVersionConflict
	}
	if errors.Is(err, repository.ErrInvalidCursor) {
		return ErrInvalidCursor
	}
	return fmt.Errorf("%s: %w", operation, err)
}
