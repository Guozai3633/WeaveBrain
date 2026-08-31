package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

var (
	ErrInvalidAISettings          = errors.New("invalid ai settings")
	ErrAISettingsConflict         = errors.New("ai settings version conflict")
	ErrReorganizeAIMemoryDisabled = errors.New("ai memory organizing is disabled")
)

// UpdateAISettingsInput is the validated patch for a user's AI settings.
// Pointer fields are applied only when present in the request (partial update).
// ExpectedRevision is the client's last-seen revision for optimistic
// concurrency; a mismatch yields ErrAISettingsConflict.
type UpdateAISettingsInput struct {
	UserID              uuid.UUID
	ExpectedRevision    int64
	AIMemoryEnabled     *bool
	AICompletionEnabled *bool
	SpeechToTextEnabled *bool
	CloudTextAllowed    *bool
	CloudAudioAllowed   *bool
}

// AISettingsService manages per-user AI consent switches. When AI memory
// organizing is switched off, any still-queued enrichment events for the user
// are cancelled so no AI call happens retroactively.
type AISettingsService struct {
	settings repository.UserAISettingsRepository
	outbox   repository.OutboxRepository
}

// NewAISettingsService creates a new AISettingsService. The outbox repository
// may be nil; cancellation of queued events is then skipped.
func NewAISettingsService(settings repository.UserAISettingsRepository, outbox repository.OutboxRepository) *AISettingsService {
	return &AISettingsService{settings: settings, outbox: outbox}
}

// Get returns the effective settings for a user, materializing the
// privacy-first defaults when the user has no row yet.
func (s *AISettingsService) Get(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error) {
	if s == nil || s.settings == nil {
		return nil, fmt.Errorf("ai settings repository is not configured")
	}
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidAISettings)
	}
	stored, err := s.settings.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user ai settings: %w", err)
	}
	if stored == nil {
		return entity.DefaultUserAISettings(userID), nil
	}
	return stored, nil
}

// Update applies a partial patch to the user's settings under optimistic
// concurrency. For a brand-new user (no row yet) the client must send
// ExpectedRevision 0; the row is then created with revision 1.
func (s *AISettingsService) Update(
	ctx context.Context,
	input UpdateAISettingsInput,
) (*entity.UserAISettings, error) {
	if s == nil || s.settings == nil {
		return nil, fmt.Errorf("ai settings repository is not configured")
	}
	if input.UserID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidAISettings)
	}

	existing, err := s.Get(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if input.ExpectedRevision != existing.Revision {
		return nil, fmt.Errorf(
			"%w: expected=%d current=%d",
			ErrAISettingsConflict,
			input.ExpectedRevision,
			existing.Revision,
		)
	}

	wasEnabled := existing.AIMemoryEnabled
	if existing.CreatedAt.IsZero() {
		// No persisted row yet: materialize the defaults and insert.
		existing = entity.DefaultUserAISettings(input.UserID)
	}
	applyAISettingsPatch(existing, input)

	if existing.CreatedAt.IsZero() {
		if err := s.settings.Create(ctx, existing); err != nil {
			if errors.Is(err, repository.ErrAISettingsVersionConflict) {
				return nil, ErrAISettingsConflict
			}
			return nil, fmt.Errorf("create user ai settings: %w", err)
		}
		updated, err := s.Get(ctx, input.UserID)
		if err != nil {
			return nil, err
		}
		s.cancelQueuedIfDisabled(ctx, input.UserID, wasEnabled, updated.AIMemoryEnabled)
		return updated, nil
	}

	newRevision, err := s.settings.Update(ctx, existing, input.ExpectedRevision)
	if err != nil {
		if errors.Is(err, repository.ErrAISettingsVersionConflict) {
			return nil, ErrAISettingsConflict
		}
		return nil, fmt.Errorf("update user ai settings: %w", err)
	}
	existing.Revision = newRevision
	s.cancelQueuedIfDisabled(ctx, input.UserID, wasEnabled, existing.AIMemoryEnabled)
	return existing, nil
}

// CountPendingReorganize returns how many of the user's ready outbox events
// were marked without AI enrichment (created while AI organizing was off) and
// are therefore eligible for an explicit backfill. When the outbox repository
// is not configured the count is 0 (no reorganize capability).
func (s *AISettingsService) CountPendingReorganize(ctx context.Context, userID uuid.UUID) (int64, error) {
	if s == nil || s.settings == nil {
		return 0, fmt.Errorf("ai settings repository is not configured")
	}
	if userID == uuid.Nil {
		return 0, fmt.Errorf("%w: user_id is required", ErrInvalidAISettings)
	}
	if s.outbox == nil {
		return 0, nil
	}
	count, err := s.outbox.CountPendingReorganize(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("count pending reorganize: %w", err)
	}
	return count, nil
}

// Reorganize explicitly re-enqueues the user's memories that were created
// while AI organizing was off so the worker backfills them with AI enrichment.
// It only runs when AI memory organizing is currently enabled; the queued
// events are stamped with a fresh policy snapshot of the current settings.
func (s *AISettingsService) Reorganize(ctx context.Context, userID uuid.UUID) (int64, error) {
	if s == nil || s.settings == nil {
		return 0, fmt.Errorf("ai settings repository is not configured")
	}
	if userID == uuid.Nil {
		return 0, fmt.Errorf("%w: user_id is required", ErrInvalidAISettings)
	}
	if s.outbox == nil {
		return 0, nil
	}
	settings, err := s.Get(ctx, userID)
	if err != nil {
		return 0, err
	}
	if !settings.AIMemoryEnabled {
		return 0, ErrReorganizeAIMemoryDisabled
	}
	policyJSON, err := json.Marshal(entity.FromAISettings(settings))
	if err != nil {
		return 0, fmt.Errorf("encode policy snapshot: %w", err)
	}
	reorganized, err := s.outbox.ReorganizeByUser(ctx, userID, policyJSON)
	if err != nil {
		return 0, fmt.Errorf("reorganize past memories: %w", err)
	}
	return reorganized, nil
}

// cancelQueuedIfDisabled drops a user's still-queued enrichment events when AI
// memory organizing transitions from enabled to disabled, so no AI call is
// made retroactively for captures created while the switch was on. The
// settings write has already succeeded; cancellation is best-effort cleanup.
func (s *AISettingsService) cancelQueuedIfDisabled(
	ctx context.Context,
	userID uuid.UUID,
	wasEnabled, nowEnabled bool,
) {
	if s.outbox == nil || !wasEnabled || nowEnabled {
		return
	}
	if _, err := s.outbox.CancelByUser(ctx, userID); err != nil {
		log.Printf("ai settings: cancel queued outbox events for user %s: %v", userID, err)
	}
}

func applyAISettingsPatch(s *entity.UserAISettings, input UpdateAISettingsInput) {
	if input.AIMemoryEnabled != nil {
		s.AIMemoryEnabled = *input.AIMemoryEnabled
	}
	if input.AICompletionEnabled != nil {
		s.AICompletionEnabled = *input.AICompletionEnabled
	}
	if input.SpeechToTextEnabled != nil {
		s.SpeechToTextEnabled = *input.SpeechToTextEnabled
	}
	if input.CloudTextAllowed != nil {
		s.CloudTextAllowed = *input.CloudTextAllowed
	}
	if input.CloudAudioAllowed != nil {
		s.CloudAudioAllowed = *input.CloudAudioAllowed
	}
}
