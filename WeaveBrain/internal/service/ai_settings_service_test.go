package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type fakeAISettingsRepo struct {
	stored         *entity.UserAISettings
	createConflict bool
	updateConflict bool
}

func (f *fakeAISettingsRepo) GetByUserID(
	_ context.Context,
	userID uuid.UUID,
) (*entity.UserAISettings, error) {
	if f.stored == nil {
		return nil, nil
	}
	cp := *f.stored
	return &cp, nil
}

func (f *fakeAISettingsRepo) Create(_ context.Context, s *entity.UserAISettings) error {
	if f.createConflict {
		return repository.ErrAISettingsVersionConflict
	}
	cp := *s
	cp.Revision = 1
	now := time.Now().UTC()
	cp.CreatedAt = now
	cp.UpdatedAt = now
	f.stored = &cp
	return nil
}

func (f *fakeAISettingsRepo) Update(
	_ context.Context,
	s *entity.UserAISettings,
	expectedRevision int64,
) (int64, error) {
	if f.updateConflict {
		return 0, repository.ErrAISettingsVersionConflict
	}
	cp := *s
	cp.Revision = expectedRevision + 1
	cp.UpdatedAt = time.Now().UTC()
	f.stored = &cp
	return cp.Revision, nil
}

func TestAISettingsGetMaterializesDefaultsWhenNoRow(t *testing.T) {
	userID := uuid.New()
	svc := NewAISettingsService(&fakeAISettingsRepo{}, nil)

	got, err := svc.Get(context.Background(), userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.UserID != userID {
		t.Fatalf("unexpected user_id %v", got.UserID)
	}
	if got.Revision != 0 {
		t.Fatalf("expected default revision 0, got %d", got.Revision)
	}
	if got.AIMemoryEnabled || got.AICompletionEnabled ||
		got.SpeechToTextEnabled || got.CloudTextAllowed || got.CloudAudioAllowed {
		t.Fatalf("defaults must be all-false, got %#v", got)
	}
}

func TestAISettingsGetReturnsStoredRow(t *testing.T) {
	userID := uuid.New()
	stored := &entity.UserAISettings{
		UserID:          userID,
		AIMemoryEnabled: true,
		Revision:        7,
		CreatedAt:       time.Now().UTC(),
	}
	svc := NewAISettingsService(&fakeAISettingsRepo{stored: stored}, nil)

	got, err := svc.Get(context.Background(), userID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.AIMemoryEnabled || got.Revision != 7 {
		t.Fatalf("expected stored values, got %#v", got)
	}
}

func TestAISettingsUpdateNewUserCreatesRowAtRevisionOne(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{}
	svc := NewAISettingsService(repo, nil)
	enabled := true

	got, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 0,
		AIMemoryEnabled:  &enabled,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !got.AIMemoryEnabled {
		t.Fatalf("expected ai_memory_enabled true, got %#v", got)
	}
	if got.Revision != 1 {
		t.Fatalf("expected revision 1 after create, got %d", got.Revision)
	}
	if repo.stored == nil {
		t.Fatal("expected row to be created")
	}
}

func TestAISettingsUpdateMatchingRevisionBumpsRevision(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:    userID,
		Revision:  2,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}}
	svc := NewAISettingsService(repo, nil)
	enabled := true

	got, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 2,
		AIMemoryEnabled:  &enabled,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Revision != 3 {
		t.Fatalf("expected revision 3, got %d", got.Revision)
	}
	if !got.AIMemoryEnabled {
		t.Fatalf("expected ai_memory_enabled true, got %#v", got)
	}
}

func TestAISettingsUpdateStaleRevisionReturnsVersionConflict(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:    userID,
		Revision:  5,
		CreatedAt: time.Now().UTC(),
	}}
	svc := NewAISettingsService(repo, nil)
	enabled := true

	_, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 3,
		AIMemoryEnabled:  &enabled,
	})
	if !errors.Is(err, ErrAISettingsConflict) {
		t.Fatalf("expected ErrAISettingsConflict, got %v", err)
	}
}

func TestAISettingsUpdatePartialPatchPreservesUnsetFields(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:              userID,
		AIMemoryEnabled:     true,
		SpeechToTextEnabled: true,
		Revision:            1,
		CreatedAt:           time.Now().UTC(),
	}}
	svc := NewAISettingsService(repo, nil)
	cloudText := true

	got, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 1,
		CloudTextAllowed: &cloudText,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !got.AIMemoryEnabled {
		t.Fatal("AIMemoryEnabled must be preserved")
	}
	if !got.SpeechToTextEnabled {
		t.Fatal("SpeechToTextEnabled must be preserved")
	}
	if !got.CloudTextAllowed {
		t.Fatal("CloudTextAllowed must be applied")
	}
	if got.CloudAudioAllowed {
		t.Fatal("CloudAudioAllowed must remain false")
	}
}

func TestAISettingsUpdateRepositoryConflictMapsToServiceConflict(t *testing.T) {
	repo := &fakeAISettingsRepo{createConflict: true}
	svc := NewAISettingsService(repo, nil)
	enabled := true

	_, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           uuid.New(),
		ExpectedRevision: 0,
		AIMemoryEnabled:  &enabled,
	})
	if !errors.Is(err, ErrAISettingsConflict) {
		t.Fatalf("expected ErrAISettingsConflict, got %v", err)
	}
}

func TestAISettingsUpdateRequiresUserID(t *testing.T) {
	svc := NewAISettingsService(&fakeAISettingsRepo{}, nil)
	enabled := true
	_, err := svc.Update(context.Background(), UpdateAISettingsInput{
		ExpectedRevision: 0,
		AIMemoryEnabled:  &enabled,
	})
	if !errors.Is(err, ErrInvalidAISettings) {
		t.Fatalf("expected ErrInvalidAISettings, got %v", err)
	}
}

func TestAISettingsGetRequiresUserID(t *testing.T) {
	svc := NewAISettingsService(&fakeAISettingsRepo{}, nil)
	_, err := svc.Get(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidAISettings) {
		t.Fatalf("expected ErrInvalidAISettings, got %v", err)
	}
}

func TestAISettingsUpdateRepositoryUpdateConflictMapsToServiceConflict(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{
		stored: &entity.UserAISettings{
			UserID:    userID,
			Revision:  1,
			CreatedAt: time.Now().UTC(),
		},
		updateConflict: true,
	}
	svc := NewAISettingsService(repo, nil)
	enabled := true

	_, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 1,
		AIMemoryEnabled:  &enabled,
	})
	if !errors.Is(err, ErrAISettingsConflict) {
		t.Fatalf("expected ErrAISettingsConflict, got %v", err)
	}
}

func TestAISettingsDisableAIMemoryCancelsQueuedEvents(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:          userID,
		AIMemoryEnabled: true,
		Revision:        1,
		CreatedAt:       time.Now().UTC(),
	}}
	outbox := &fakeOutboxRepo{}
	svc := NewAISettingsService(repo, outbox)
	enabled := false

	got, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 1,
		AIMemoryEnabled:  &enabled,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.AIMemoryEnabled {
		t.Fatal("ai_memory_enabled must be off after update")
	}
	if len(outbox.cancelled) != 1 || outbox.cancelled[0] != userID {
		t.Fatalf("expected queued events cancelled for the user, got %#v", outbox.cancelled)
	}
}

func TestAISettingsEnableAIMemoryDoesNotCancel(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:    userID,
		Revision:  1,
		CreatedAt: time.Now().UTC(),
	}}
	outbox := &fakeOutboxRepo{}
	svc := NewAISettingsService(repo, outbox)
	enabled := true

	if _, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 1,
		AIMemoryEnabled:  &enabled,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(outbox.cancelled) != 0 {
		t.Fatalf("enabling AI must not cancel events, got %#v", outbox.cancelled)
	}
}

func TestAISettingsPatchUnrelatedSwitchDoesNotCancel(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:              userID,
		AIMemoryEnabled:     true,
		SpeechToTextEnabled: false,
		Revision:            1,
		CreatedAt:           time.Now().UTC(),
	}}
	outbox := &fakeOutboxRepo{}
	svc := NewAISettingsService(repo, outbox)
	cloudText := true

	if _, err := svc.Update(context.Background(), UpdateAISettingsInput{
		UserID:           userID,
		ExpectedRevision: 1,
		CloudTextAllowed: &cloudText,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(outbox.cancelled) != 0 {
		t.Fatalf("a non-AI-memory switch change must not cancel, got %#v", outbox.cancelled)
	}
}

func TestAISettingsReorganizeWhenAIMemoryDisabled(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:    userID,
		Revision:  1,
		CreatedAt: time.Now().UTC(),
	}}
	outbox := &fakeOutboxRepo{}
	svc := NewAISettingsService(repo, outbox)

	_, err := svc.Reorganize(context.Background(), userID)
	if !errors.Is(err, ErrReorganizeAIMemoryDisabled) {
		t.Fatalf("expected ErrReorganizeAIMemoryDisabled, got %v", err)
	}
	if len(outbox.reorganized) != 0 {
		t.Fatalf("must not re-enqueue when AI memory organizing is off, got %#v", outbox.reorganized)
	}
}

func TestAISettingsReorganizeDelegatesWithCurrentSnapshot(t *testing.T) {
	userID := uuid.New()
	repo := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:              userID,
		AIMemoryEnabled:     true,
		AICompletionEnabled: true,
		Revision:            2,
		CreatedAt:           time.Now().UTC(),
	}}
	outbox := &fakeOutboxRepo{}
	svc := NewAISettingsService(repo, outbox)

	count, err := svc.Reorganize(context.Background(), userID)
	if err != nil {
		t.Fatalf("Reorganize: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 re-enqueued, got %d", count)
	}
	if len(outbox.reorganized) != 1 || outbox.reorganized[0].userID != userID {
		t.Fatalf("expected reorganize for the user, got %#v", outbox.reorganized)
	}
	var snapshot entity.PolicySnapshot
	if err := json.Unmarshal(outbox.reorganized[0].policy, &snapshot); err != nil {
		t.Fatalf("decode policy snapshot: %v", err)
	}
	if !snapshot.AIMemoryEnabled || !snapshot.AICompletionEnabled {
		t.Fatalf("snapshot must carry current switches, got %#v", snapshot)
	}
}

func TestAISettingsCountPendingReorganizeWithoutOutboxIsZero(t *testing.T) {
	userID := uuid.New()
	svc := NewAISettingsService(&fakeAISettingsRepo{}, nil)

	count, err := svc.CountPendingReorganize(context.Background(), userID)
	if err != nil {
		t.Fatalf("CountPendingReorganize: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 when outbox is unavailable, got %d", count)
	}
}

func TestAISettingsReorganizeWithoutOutboxIsZero(t *testing.T) {
	userID := uuid.New()
	svc := NewAISettingsService(&fakeAISettingsRepo{}, nil)

	count, err := svc.Reorganize(context.Background(), userID)
	if err != nil {
		t.Fatalf("Reorganize: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 when outbox is unavailable, got %d", count)
	}
}
