package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type captureKey struct {
	userID    uuid.UUID
	captureID uuid.UUID
}

type memoryCaptureRepository struct {
	items        map[captureKey]*entity.CaptureAggregate
	revisions    map[captureKey][]*entity.EnrichmentRevision
	createCalls  int
	lastPolicy   *entity.PolicySnapshot
	lastRevision *entity.EnrichmentRevision
}

func newMemoryCaptureRepository() *memoryCaptureRepository {
	return &memoryCaptureRepository{
		items:     make(map[captureKey]*entity.CaptureAggregate),
		revisions: make(map[captureKey][]*entity.EnrichmentRevision),
	}
}

func (r *memoryCaptureRepository) Create(
	_ context.Context,
	capture *entity.Capture,
	card *entity.MemoryCard,
	rev *entity.EnrichmentRevision,
	policy *entity.PolicySnapshot,
) (bool, error) {
	r.createCalls++
	r.lastPolicy = policy
	r.lastRevision = rev
	key := captureKey{userID: capture.UserID, captureID: capture.ID}
	if existing, ok := r.items[key]; ok {
		if existing.Capture.RequestHash != capture.RequestHash {
			return false, repository.ErrCaptureIdempotencyConflict
		}
		return true, nil
	}

	now := time.Now().UTC()
	storedCapture := *capture
	storedCapture.Version = 1
	storedCapture.LifecycleStatus = "active"
	storedCapture.CreatedAt = now
	storedCapture.UpdatedAt = now
	storedCard := *card
	storedCard.CreatedAt = now
	storedCard.UpdatedAt = now
	r.items[key] = &entity.CaptureAggregate{
		Capture:    &storedCapture,
		MemoryCard: &storedCard,
	}
	if rev != nil {
		revision := *rev
		revision.Revision = 1
		revision.CardVersion = storedCard.Version
		revision.CreatedAt = now
		r.revisions[key] = append(r.revisions[key], &revision)
	}
	return false, nil
}

func (r *memoryCaptureRepository) GetByID(
	_ context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) (*entity.CaptureAggregate, error) {
	item, ok := r.items[captureKey{userID: userID, captureID: captureID}]
	if !ok {
		return nil, repository.ErrCaptureNotFound
	}
	return item, nil
}

func (r *memoryCaptureRepository) UpdateCardTitle(
	_ context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	title string,
) error {
	item, ok := r.items[captureKey{userID: userID, captureID: captureID}]
	if !ok {
		return repository.ErrCaptureNotFound
	}
	item.MemoryCard.Title = title
	return nil
}

func (r *memoryCaptureRepository) FindExternalDuplicate(
	_ context.Context,
	userID uuid.UUID,
	sourceName, externalID string,
	excludeCaptureID uuid.UUID,
) (*uuid.UUID, error) {
	for key, agg := range r.items {
		if key.userID != userID || key.captureID == excludeCaptureID {
			continue
		}
		c := agg.Capture
		if c == nil || c.LifecycleStatus == "deleted" || c.SourceName == nil || c.ExternalID == nil {
			continue
		}
		if *c.SourceName == sourceName && *c.ExternalID == externalID {
			id := c.ID
			return &id, nil
		}
	}
	return nil, nil
}

func (r *memoryCaptureRepository) FindContentHashMatch(
	_ context.Context,
	userID uuid.UUID,
	contentHash string,
	excludeCaptureID uuid.UUID,
) (*uuid.UUID, error) {
	for key, agg := range r.items {
		if key.userID != userID || key.captureID == excludeCaptureID {
			continue
		}
		c := agg.Capture
		if c == nil || c.LifecycleStatus == "deleted" || c.ContentHash == nil {
			continue
		}
		if *c.ContentHash == contentHash {
			id := c.ID
			return &id, nil
		}
	}
	return nil, nil
}

func TestCaptureServiceCreateWithoutProjectAndReplay100Times(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)
	userID := uuid.New()
	captureID := uuid.New()
	text := "  跑步时想到：把零散灵感先安全保存，再异步整理成长期记忆。原文空格也要保留。  "
	input := CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindText,
		Text:          text,
		Source:        "mobile_android",
		ClientVersion: 1,
	}

	for attempt := 0; attempt < 100; attempt++ {
		result, err := captureService.Create(context.Background(), userID, input)
		if err != nil {
			t.Fatalf("attempt %d: create capture: %v", attempt+1, err)
		}
		if got, want := result.Replayed, attempt > 0; got != want {
			t.Fatalf("attempt %d: replayed=%v, want %v", attempt+1, got, want)
		}
	}

	if got := len(repo.items); got != 1 {
		t.Fatalf("expected one stored aggregate after 100 requests, got %d", got)
	}
	stored := repo.items[captureKey{userID: userID, captureID: captureID}]
	if stored.Capture.CollectionID != nil {
		t.Fatalf("expected capture without project/collection, got %v", stored.Capture.CollectionID)
	}
	if stored.Capture.OriginalText == nil || *stored.Capture.OriginalText != text {
		t.Fatalf("expected exact original text to be preserved, got %#v", stored.Capture.OriginalText)
	}
	if got := utf8.RuneCountInString(stored.MemoryCard.Title); got > fallbackTitleMaxRunes {
		t.Fatalf("fallback title has %d runes, max is %d", got, fallbackTitleMaxRunes)
	}
	if stored.MemoryCard.ProcessingStatus != "ready" {
		t.Fatalf("expected synchronous fallback card to be ready, got %q", stored.MemoryCard.ProcessingStatus)
	}
	revs := repo.revisions[captureKey{userID: userID, captureID: captureID}]
	if len(revs) != 1 {
		t.Fatalf("expected one fallback revision, got %d", len(revs))
	}
	if revs[0].Source != entity.EnrichmentSourceFallback {
		t.Fatalf("expected fallback revision source, got %q", revs[0].Source)
	}
	if revs[0].CardVersion != stored.MemoryCard.Version {
		t.Fatalf("expected revision card_version to match card version, got %d", revs[0].CardVersion)
	}
}

func TestCaptureServiceIdempotencyConflict(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)
	userID := uuid.New()
	captureID := uuid.New()
	base := CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindText,
		Text:          "first",
		Source:        "web",
		ClientVersion: 1,
	}
	if _, err := captureService.Create(context.Background(), userID, base); err != nil {
		t.Fatalf("first create: %v", err)
	}

	base.Text = "different"
	_, err := captureService.Create(context.Background(), userID, base)
	if !errors.Is(err, ErrCaptureIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
	if got := len(repo.items); got != 1 {
		t.Fatalf("conflict must not add data, got %d items", got)
	}
}

func TestCaptureServiceGetIsUserScoped(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)
	ownerID := uuid.New()
	otherUserID := uuid.New()
	captureID := uuid.New()
	_, err := captureService.Create(context.Background(), ownerID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindText,
		Text:          "private memory",
		ClientVersion: 1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := captureService.GetByID(context.Background(), ownerID, captureID); err != nil {
		t.Fatalf("owner should read capture: %v", err)
	}
	if _, err := captureService.GetByID(context.Background(), otherUserID, captureID); !errors.Is(err, ErrCaptureNotFound) {
		t.Fatalf("other user must see not found, got %v", err)
	}
}

func TestCaptureServiceRejectsInvalidR1Input(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	tests := []struct {
		name  string
		input CreateCaptureInput
	}{
		{
			name: "empty text",
			input: CreateCaptureInput{
				ID: captureID, Kind: entity.CaptureKindText, Text: "  ", ClientVersion: 1,
			},
		},
		{
			name: "invalid client version",
			input: CreateCaptureInput{
				ID: captureID, Kind: entity.CaptureKindText, Text: "text", ClientVersion: 0,
			},
		},
		{
			name: "text too long",
			input: CreateCaptureInput{
				ID: captureID, Kind: entity.CaptureKindText,
				Text: strings.Repeat("想", MaxCaptureTextRunes+1), ClientVersion: 1,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			captureService := NewCaptureService(newMemoryCaptureRepository(), nil)
			_, err := captureService.Create(context.Background(), userID, test.input)
			if !errors.Is(err, ErrInvalidCapture) {
				t.Fatalf("expected invalid capture, got %v", err)
			}
		})
	}
}

func TestCaptureServiceCreateAudioCaptureWithEmptyText(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)
	userID := uuid.New()
	captureID := uuid.New()

	result, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindAudio,
		Text:          "",
		Source:        "mobile_android",
		ClientVersion: 1,
	})
	if err != nil {
		t.Fatalf("audio capture should be allowed: %v", err)
	}
	if result.Replayed {
		t.Fatalf("expected a fresh audio capture, got replayed")
	}
	if result.Aggregate.Capture.Kind != entity.CaptureKindAudio {
		t.Fatalf("expected kind audio, got %q", result.Aggregate.Capture.Kind)
	}
	if result.Aggregate.Capture.OriginalText != nil {
		t.Fatalf("expected nil original_text for audio capture, got %#v", result.Aggregate.Capture.OriginalText)
	}
	if got := result.Aggregate.MemoryCard.Title; got != "语音记录" {
		t.Fatalf("expected fallback title 语音记录, got %q", got)
	}
	if got := result.Aggregate.MemoryCard.ProcessingStatus; got != "ready" {
		t.Fatalf("expected audio capture card to be ready, got %q", got)
	}
}

func TestCaptureServiceRejectsUnsupportedKind(t *testing.T) {
	captureService := NewCaptureService(newMemoryCaptureRepository(), nil)
	_, err := captureService.Create(context.Background(), uuid.New(), CreateCaptureInput{
		ID:            uuid.New(),
		Kind:          "video",
		Text:          "hello",
		ClientVersion: 1,
	})
	if !errors.Is(err, ErrInvalidCapture) {
		t.Fatalf("expected invalid capture for unsupported kind, got %v", err)
	}
}

type fakeSettingsSource struct {
	settings *entity.UserAISettings
	err      error
}

func (f *fakeSettingsSource) Get(context.Context, uuid.UUID) (*entity.UserAISettings, error) {
	return f.settings, f.err
}

func TestCaptureServiceSnapshotsAISettingsOnCreate(t *testing.T) {
	repo := newMemoryCaptureRepository()
	userID := uuid.New()
	settings := entity.DefaultUserAISettings(userID)
	settings.AIMemoryEnabled = true
	settings.CloudTextAllowed = true
	captureService := NewCaptureService(repo, &fakeSettingsSource{settings: settings})

	if _, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
		ID:            uuid.New(),
		Kind:          entity.CaptureKindText,
		Text:          "snapshot me",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create capture: %v", err)
	}
	if repo.lastPolicy == nil {
		t.Fatal("expected policy snapshot to be recorded")
	}
	if !repo.lastPolicy.AIMemoryEnabled {
		t.Fatal("snapshot must record ai_memory_enabled=true from settings")
	}
	if !repo.lastPolicy.CloudTextAllowed {
		t.Fatal("snapshot must record cloud_text_allowed=true from settings")
	}
}

func TestCaptureServiceFallsBackToPrivacyFirstWhenSettingsSourceFails(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, &fakeSettingsSource{err: errWorkerTest("settings down")})

	if _, err := captureService.Create(context.Background(), uuid.New(), CreateCaptureInput{
		ID:            uuid.New(),
		Kind:          entity.CaptureKindText,
		Text:          "must still save",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("capture must not be blocked by settings outage: %v", err)
	}
	if repo.lastPolicy == nil || repo.lastPolicy.AIMemoryEnabled {
		t.Fatalf("expected all-disabled fallback snapshot, got %#v", repo.lastPolicy)
	}
}

func TestCaptureServiceFallsBackToPrivacyFirstWhenNoSettingsSource(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)

	if _, err := captureService.Create(context.Background(), uuid.New(), CreateCaptureInput{
		ID:            uuid.New(),
		Kind:          entity.CaptureKindText,
		Text:          "default privacy",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create capture: %v", err)
	}
	if repo.lastPolicy == nil || repo.lastPolicy.AIMemoryEnabled {
		t.Fatalf("expected all-disabled default snapshot, got %#v", repo.lastPolicy)
	}
}

func TestCaptureServiceImportNormalization(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()

	t.Run("rejects empty import text", func(t *testing.T) {
		captureService := NewCaptureService(newMemoryCaptureRepository(), nil)
		_, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
			ID:            captureID,
			Kind:          entity.CaptureKindImport,
			Text:          "   ",
			ClientVersion: 1,
		})
		if !errors.Is(err, ErrInvalidCapture) {
			t.Fatalf("expected invalid capture for empty import text, got %v", err)
		}
	})

	t.Run("rejects oversized import text", func(t *testing.T) {
		captureService := NewCaptureService(newMemoryCaptureRepository(), nil)
		_, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
			ID:            captureID,
			Kind:          entity.CaptureKindImport,
			Text:          strings.Repeat("导", MaxCaptureTextRunes+1),
			ClientVersion: 1,
		})
		if !errors.Is(err, ErrInvalidCapture) {
			t.Fatalf("expected invalid capture for oversized import text, got %v", err)
		}
	})

	t.Run("rejects oversized external_id", func(t *testing.T) {
		captureService := NewCaptureService(newMemoryCaptureRepository(), nil)
		externalID := strings.Repeat("x", 201)
		_, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
			ID:            captureID,
			Kind:          entity.CaptureKindImport,
			Text:          "content",
			ExternalID:    &externalID,
			SourceName:    strPtr("旧备忘录"),
			ClientVersion: 1,
		})
		if !errors.Is(err, ErrInvalidCapture) {
			t.Fatalf("expected invalid capture for oversized external_id, got %v", err)
		}
	})

	t.Run("rejects oversized source_name", func(t *testing.T) {
		captureService := NewCaptureService(newMemoryCaptureRepository(), nil)
		sourceName := strings.Repeat("源", 101)
		_, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
			ID:            captureID,
			Kind:          entity.CaptureKindImport,
			Text:          "content",
			SourceName:    &sourceName,
			ClientVersion: 1,
		})
		if !errors.Is(err, ErrInvalidCapture) {
			t.Fatalf("expected invalid capture for oversized source_name, got %v", err)
		}
	})

	t.Run("defaults source to import", func(t *testing.T) {
		repo := newMemoryCaptureRepository()
		captureService := NewCaptureService(repo, nil)
		result, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
			ID:            captureID,
			Kind:          entity.CaptureKindImport,
			Text:          "  导入的第一条笔记  ",
			ClientVersion: 1,
		})
		if err != nil {
			t.Fatalf("create import capture: %v", err)
		}
		if got := result.Aggregate.Capture.Source; got != "import" {
			t.Fatalf("expected default source import, got %q", got)
		}
	})
}

func TestCaptureServiceImportOverrides(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)
	userID := uuid.New()
	captureID := uuid.New()
	title := "自定义标题"
	primaryType := "note"
	tags := []string{"a", "b"}

	result, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
		ID:                  captureID,
		Kind:                entity.CaptureKindImport,
		Text:                "原始正文内容",
		ExternalID:          strPtr("t1"),
		SourceName:          strPtr("旧备忘录"),
		TitleOverride:       &title,
		PrimaryTypeOverride: &primaryType,
		TagsOverride:        tags,
		ClientVersion:       1,
	})
	if err != nil {
		t.Fatalf("create import capture: %v", err)
	}
	card := result.Aggregate.MemoryCard
	if card.Title != title {
		t.Fatalf("expected override title %q, got %q", title, card.Title)
	}
	if card.PrimaryType != primaryType {
		t.Fatalf("expected override primary type %q, got %q", primaryType, card.PrimaryType)
	}
	if len(card.Tags) != 2 || card.Tags[0] != "a" || card.Tags[1] != "b" {
		t.Fatalf("expected override tags, got %#v", card.Tags)
	}
	c := result.Aggregate.Capture
	if c.ExternalID == nil || *c.ExternalID != "t1" {
		t.Fatalf("expected external_id t1, got %#v", c.ExternalID)
	}
	if c.SourceName == nil || *c.SourceName != "旧备忘录" {
		t.Fatalf("expected source_name 旧备忘录, got %#v", c.SourceName)
	}
	if c.ContentHash == nil || *c.ContentHash == "" {
		t.Fatalf("expected content_hash to be set for import, got %#v", c.ContentHash)
	}

	revs := repo.revisions[captureKey{userID: userID, captureID: captureID}]
	if len(revs) != 1 {
		t.Fatalf("expected one initial revision, got %d", len(revs))
	}
	if revs[0].Source != entity.EnrichmentSourceImport {
		t.Fatalf("expected initial revision source import, got %q", revs[0].Source)
	}
	if provenance, ok := revs[0].Provenance["source"]; !ok || provenance != "import" {
		t.Fatalf("expected provenance source import, got %#v", revs[0].Provenance)
	}
}

func TestCaptureServiceImportDuplicateExternalID(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)
	userID := uuid.New()
	firstID := uuid.New()

	if _, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
		ID:            firstID,
		Kind:          entity.CaptureKindImport,
		Text:          "first",
		ExternalID:    strPtr("t1"),
		SourceName:    strPtr("旧备忘录"),
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create first import: %v", err)
	}

	secondID := uuid.New()
	_, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
		ID:            secondID,
		Kind:          entity.CaptureKindImport,
		Text:          "first but reimported",
		ExternalID:    strPtr("t1"),
		SourceName:    strPtr("旧备忘录"),
		ClientVersion: 1,
	})
	if !errors.Is(err, ErrDuplicateExternalID) {
		t.Fatalf("expected duplicate external id error, got %v", err)
	}
	var dupErr *DuplicateExternalIDError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected DuplicateExternalIDError, got %T", err)
	}
	if dupErr.ExistingCaptureID != firstID {
		t.Fatalf("expected existing capture id %s, got %s", firstID, dupErr.ExistingCaptureID)
	}
	if got := len(repo.items); got != 1 {
		t.Fatalf("duplicate must not add data, got %d items", got)
	}
}

func TestCaptureServiceImportContentHashDedupe(t *testing.T) {
	repo := newMemoryCaptureRepository()
	captureService := NewCaptureService(repo, nil)
	userID := uuid.New()
	firstID := uuid.New()

	if _, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
		ID:            firstID,
		Kind:          entity.CaptureKindImport,
		Text:          "  相同  内容  ",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create first import: %v", err)
	}

	secondID := uuid.New()
	result, err := captureService.Create(context.Background(), userID, CreateCaptureInput{
		ID:            secondID,
		Kind:          entity.CaptureKindImport,
		Text:          "相同 内容", // whitespace-collapsed hash matches
		ClientVersion: 1,
	})
	if err != nil {
		t.Fatalf("content-hash duplicate must still import: %v", err)
	}
	if result.Dedupe == nil || result.Dedupe.Status != "suggested" {
		t.Fatalf("expected dedupe suggested, got %#v", result.Dedupe)
	}
	if result.Dedupe.ExistingCaptureID == nil || *result.Dedupe.ExistingCaptureID != firstID {
		t.Fatalf("expected dedupe existing id %s, got %#v", firstID, result.Dedupe.ExistingCaptureID)
	}
	if got := len(repo.items); got != 2 {
		t.Fatalf("suspected duplicate still creates a capture, got %d items", got)
	}
}

func strPtr(s string) *string { return &s }
