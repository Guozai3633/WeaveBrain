package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type fakeMemoryRepository struct {
	listFn          func(context.Context, uuid.UUID, entity.MemoryListQuery) (*entity.MemoryListResult, error)
	listRevisionsFn func(context.Context, uuid.UUID, uuid.UUID) ([]*entity.EnrichmentRevision, error)
	appendFn        func(context.Context, uuid.UUID, uuid.UUID, *entity.EnrichmentRevision, *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error)
	appendNoteFn    func(context.Context, uuid.UUID, uuid.UUID, *entity.EnrichmentRevision) (*entity.EnrichmentRevision, *entity.MemoryCard, error)
	setPinnedFn     func(context.Context, uuid.UUID, uuid.UUID, bool) (*entity.MemoryCard, error)
	setLifecycleFn  func(context.Context, uuid.UUID, uuid.UUID, string) (*entity.CaptureAggregate, error)
}

func (f *fakeMemoryRepository) List(ctx context.Context, userID uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error) {
	if f.listFn == nil {
		return &entity.MemoryListResult{}, nil
	}
	return f.listFn(ctx, userID, q)
}

func (f *fakeMemoryRepository) ListRevisions(ctx context.Context, userID, captureID uuid.UUID) ([]*entity.EnrichmentRevision, error) {
	if f.listRevisionsFn == nil {
		return []*entity.EnrichmentRevision{}, nil
	}
	return f.listRevisionsFn(ctx, userID, captureID)
}

func (f *fakeMemoryRepository) AppendRevisionAndUpdateCard(ctx context.Context, userID, captureID uuid.UUID, rev *entity.EnrichmentRevision, card *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
	if f.appendFn == nil {
		return rev, card, nil
	}
	return f.appendFn(ctx, userID, captureID, rev, card)
}

func (f *fakeMemoryRepository) AppendNoteAndBump(ctx context.Context, userID, captureID uuid.UUID, rev *entity.EnrichmentRevision) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
	if f.appendNoteFn == nil {
		return rev, &entity.MemoryCard{}, nil
	}
	return f.appendNoteFn(ctx, userID, captureID, rev)
}

func (f *fakeMemoryRepository) SetPinned(ctx context.Context, userID, captureID uuid.UUID, pinned bool) (*entity.MemoryCard, error) {
	if f.setPinnedFn == nil {
		return &entity.MemoryCard{}, nil
	}
	return f.setPinnedFn(ctx, userID, captureID, pinned)
}

func (f *fakeMemoryRepository) SetLifecycle(ctx context.Context, userID, captureID uuid.UUID, status string) (*entity.CaptureAggregate, error) {
	if f.setLifecycleFn == nil {
		return &entity.CaptureAggregate{}, nil
	}
	return f.setLifecycleFn(ctx, userID, captureID, status)
}

func createMemoryCapture(t *testing.T, captures *memoryCaptureRepository, userID, captureID uuid.UUID, text string) {
	t.Helper()
	if _, err := NewCaptureService(captures, nil).Create(context.Background(), userID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindText,
		Text:          text,
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create capture: %v", err)
	}
}

func TestMemoryServiceListDefaultsAndValidatesLifecycle(t *testing.T) {
	userID := uuid.New()
	var gotQuery entity.MemoryListQuery
	memory := &fakeMemoryRepository{
		listFn: func(_ context.Context, u uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error) {
			gotQuery = q
			return &entity.MemoryListResult{}, nil
		},
	}
	svc := NewMemoryService(memory, newMemoryCaptureRepository(), nil, nil)

	if _, err := svc.List(context.Background(), userID, entity.MemoryListQuery{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if gotQuery.Limit != defaultMemoryPageLimit {
		t.Fatalf("expected default limit %d, got %d", defaultMemoryPageLimit, gotQuery.Limit)
	}
	if gotQuery.LifecycleStatus != "active" {
		t.Fatalf("expected default lifecycle active, got %q", gotQuery.LifecycleStatus)
	}

	_, err := svc.List(context.Background(), userID, entity.MemoryListQuery{LifecycleStatus: "trashed"})
	if !errors.Is(err, ErrInvalidMemoryInput) {
		t.Fatalf("expected invalid lifecycle error, got %v", err)
	}
}

func TestMemoryServiceListCapsLimitAndPassesThroughResult(t *testing.T) {
	userID := uuid.New()
	nextCursor := "cursor-token"
	memory := &fakeMemoryRepository{
		listFn: func(_ context.Context, u uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error) {
			if q.Limit != maxMemoryPageLimit {
				t.Fatalf("expected limit capped at %d, got %d", maxMemoryPageLimit, q.Limit)
			}
			return &entity.MemoryListResult{NextCursor: &nextCursor}, nil
		},
	}
	svc := NewMemoryService(memory, newMemoryCaptureRepository(), nil, nil)

	result, err := svc.List(context.Background(), userID, entity.MemoryListQuery{Limit: 500})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.NextCursor == nil || *result.NextCursor != nextCursor {
		t.Fatalf("expected next cursor passed through, got %#v", result.NextCursor)
	}
}

func TestMemoryServiceGetDetailWithAudioAndTranscript(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "detail text")

	now := time.Now().UTC()
	revisions := []*entity.EnrichmentRevision{
		{ID: 1, UserID: userID, CaptureID: captureID, Revision: 1, CardVersion: 1,
			Source: entity.EnrichmentSourceFallback, SourceRevision: 1,
			Changes: map[string]any{"title": "detail text"}, CreatedAt: now},
	}
	memory := &fakeMemoryRepository{
		listRevisionsFn: func(_ context.Context, gotUser, gotCapture uuid.UUID) ([]*entity.EnrichmentRevision, error) {
			if gotUser != userID || gotCapture != captureID {
				t.Fatalf("unexpected revision scope %v/%v", gotUser, gotCapture)
			}
			return revisions, nil
		},
	}
	assets := newFakeAudioAssetRepo()
	assets.items[audioAssetKey{userID, uuid.New()}] = &entity.AudioAsset{
		ID: uuid.New(), UserID: userID, CaptureID: captureID, UploadState: entity.AudioUploadStateComplete,
	}
	transcripts := newFakeTranscriptRepo()
	transcripts.revisions[transcriptKey{userID, captureID}] = []*entity.TranscriptRevision{
		{ID: 1, UserID: userID, CaptureID: captureID, Revision: 1, Text: "转写内容",
			Source: entity.TranscriptSourceSTT, CreatedAt: now},
	}
	svc := NewMemoryService(memory, captures, assets, transcripts)

	detail, err := svc.GetDetail(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("GetDetail: %v", err)
	}
	if detail.Capture.ID != captureID {
		t.Fatalf("unexpected capture %v", detail.Capture.ID)
	}
	if detail.Audio == nil {
		t.Fatal("expected audio asset in detail")
	}
	if detail.Transcript == nil || detail.Transcript.Text != "转写内容" {
		t.Fatalf("expected transcript in detail, got %#v", detail.Transcript)
	}
	if len(detail.Revisions) != 1 || detail.Revisions[0].Source != entity.EnrichmentSourceFallback {
		t.Fatalf("expected one fallback revision, got %#v", detail.Revisions)
	}
}

func TestMemoryServiceGetDetailToleratesMissingAudioAndTranscript(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "text only")

	svc := NewMemoryService(&fakeMemoryRepository{}, captures, newFakeAudioAssetRepo(), newFakeTranscriptRepo())
	detail, err := svc.GetDetail(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("GetDetail must tolerate absent audio/transcript: %v", err)
	}
	if detail.Audio != nil || detail.Transcript != nil {
		t.Fatalf("expected nil audio/transcript for text-only memory, got %#v/%#v", detail.Audio, detail.Transcript)
	}
}

func TestMemoryServiceGetDetailToleratesNilAudioTranscriptRepos(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "text only")

	svc := NewMemoryService(&fakeMemoryRepository{}, captures, nil, nil)
	detail, err := svc.GetDetail(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("GetDetail must tolerate nil audio/transcript repos: %v", err)
	}
	if detail.Audio != nil || detail.Transcript != nil {
		t.Fatalf("expected nil audio/transcript, got %#v/%#v", detail.Audio, detail.Transcript)
	}
}

func TestMemoryServiceGetDetailNotFound(t *testing.T) {
	svc := NewMemoryService(&fakeMemoryRepository{}, newMemoryCaptureRepository(), nil, nil)
	_, err := svc.GetDetail(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrMemoryNotFound) {
		t.Fatalf("expected memory not found, got %v", err)
	}
}

func TestMemoryServiceCorrectRecordsUserRevision(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "原始标题")

	var gotRev *entity.EnrichmentRevision
	var gotCard *entity.MemoryCard
	returnedRev := &entity.EnrichmentRevision{ID: 9, Revision: 2, CardVersion: 2}
	returnedCard := &entity.MemoryCard{ID: uuid.New(), Title: "修正标题", PrimaryType: "idea"}
	memory := &fakeMemoryRepository{
		appendFn: func(_ context.Context, u, c uuid.UUID, rev *entity.EnrichmentRevision, card *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
			gotRev = rev
			gotCard = card
			return returnedRev, returnedCard, nil
		},
	}
	svc := NewMemoryService(memory, captures, nil, nil)

	title := "修正标题"
	summary := "修正摘要"
	primaryType := "idea"
	tags := []string{"思考", "跑步"}
	keyPoints := []string{"要点一"}
	revision, aggregate, err := svc.Correct(context.Background(), userID, captureID, CorrectMemoryInput{
		Title: &title, Summary: &summary, PrimaryType: &primaryType, Tags: tags, KeyPoints: keyPoints,
	})
	if err != nil {
		t.Fatalf("Correct: %v", err)
	}
	if gotRev.Source != entity.EnrichmentSourceUser {
		t.Fatalf("expected user source, got %q", gotRev.Source)
	}
	if gotRev.SourceRevision != 1 {
		t.Fatalf("expected source_revision 1 (capture version), got %d", gotRev.SourceRevision)
	}
	if gotRev.Changes["title"] != "修正标题" || gotRev.Changes["summary"] != "修正摘要" ||
		gotRev.Changes["primary_type"] != "idea" {
		t.Fatalf("unexpected changes: %#v", gotRev.Changes)
	}
	if gotRev.Provenance["title"] != "user" || gotRev.Provenance["primary_type"] != "user" {
		t.Fatalf("unexpected provenance: %#v", gotRev.Provenance)
	}
	if gotCard.Title != "修正标题" || gotCard.PrimaryType != "idea" {
		t.Fatalf("unexpected updated card: %#v", gotCard)
	}
	if gotCard.Summary == nil || *gotCard.Summary != "修正摘要" {
		t.Fatalf("summary not applied, got %#v", gotCard.Summary)
	}
	if len(gotCard.Tags) != 2 || len(gotCard.KeyPoints) != 1 {
		t.Fatalf("tags/key_points not applied: %#v / %#v", gotCard.Tags, gotCard.KeyPoints)
	}
	if revision != returnedRev {
		t.Fatalf("revision must be passed through from repository")
	}
	if aggregate.MemoryCard != returnedCard {
		t.Fatalf("aggregate must use the refreshed card from repository")
	}
}

func TestMemoryServiceCorrectValidation(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "text")
	svc := NewMemoryService(&fakeMemoryRepository{}, captures, nil, nil)

	t.Run("empty title", func(t *testing.T) {
		title := "   "
		_, _, err := svc.Correct(context.Background(), userID, captureID, CorrectMemoryInput{Title: &title})
		if !errors.Is(err, ErrInvalidMemoryInput) {
			t.Fatalf("expected invalid memory input, got %v", err)
		}
	})
	t.Run("invalid primary type", func(t *testing.T) {
		primary := "bogus"
		_, _, err := svc.Correct(context.Background(), userID, captureID, CorrectMemoryInput{PrimaryType: &primary})
		if !errors.Is(err, ErrInvalidMemoryInput) {
			t.Fatalf("expected invalid memory input, got %v", err)
		}
	})
	t.Run("nothing to correct", func(t *testing.T) {
		_, _, err := svc.Correct(context.Background(), userID, captureID, CorrectMemoryInput{})
		if !errors.Is(err, ErrInvalidMemoryInput) {
			t.Fatalf("expected invalid memory input, got %v", err)
		}
	})
	t.Run("tags too many", func(t *testing.T) {
		tags := make([]string, maxMemoryTags+1)
		for i := range tags {
			tags[i] = "tag"
		}
		_, _, err := svc.Correct(context.Background(), userID, captureID, CorrectMemoryInput{Tags: tags})
		if !errors.Is(err, ErrInvalidMemoryInput) {
			t.Fatalf("expected invalid memory input, got %v", err)
		}
	})
}

func TestMemoryServiceCorrectNotFound(t *testing.T) {
	svc := NewMemoryService(&fakeMemoryRepository{}, newMemoryCaptureRepository(), nil, nil)
	title := "x"
	_, _, err := svc.Correct(context.Background(), uuid.New(), uuid.New(), CorrectMemoryInput{Title: &title})
	if !errors.Is(err, ErrMemoryNotFound) {
		t.Fatalf("expected memory not found, got %v", err)
	}
}

func TestMemoryServiceAppendNoteRecordsUserRevision(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "原始标题")

	var gotRev *entity.EnrichmentRevision
	memory := &fakeMemoryRepository{
		appendNoteFn: func(_ context.Context, u, c uuid.UUID, rev *entity.EnrichmentRevision) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
			gotRev = rev
			return rev, &entity.MemoryCard{Title: "原始标题", Version: 2}, nil
		},
	}
	svc := NewMemoryService(memory, captures, nil, nil)

	revision, aggregate, err := svc.AppendNote(context.Background(), userID, captureID, "  继续想   ")
	if err != nil {
		t.Fatalf("AppendNote: %v", err)
	}
	if gotRev.Source != entity.EnrichmentSourceUser {
		t.Fatalf("expected user source, got %q", gotRev.Source)
	}
	if gotRev.Changes["note"] != "继续想" {
		t.Fatalf("note must be trimmed, got %#v", gotRev.Changes["note"])
	}
	if gotRev.Provenance["note"] != "user" {
		t.Fatalf("unexpected provenance %#v", gotRev.Provenance)
	}
	if gotRev.SourceRevision != 1 {
		t.Fatalf("expected source_revision 1, got %d", gotRev.SourceRevision)
	}
	if revision != gotRev {
		t.Fatalf("revision must be passed through from repository")
	}
	if aggregate.MemoryCard.Version != 2 {
		t.Fatalf("expected bumped card version from repository, got %d", aggregate.MemoryCard.Version)
	}
}

func TestMemoryServiceAppendNoteEmptyText(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "text")
	svc := NewMemoryService(&fakeMemoryRepository{}, captures, nil, nil)

	_, _, err := svc.AppendNote(context.Background(), userID, captureID, "   ")
	if !errors.Is(err, ErrInvalidMemoryInput) {
		t.Fatalf("expected invalid memory input, got %v", err)
	}
}

func TestMemoryServiceSetPinned(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "text")

	newCard := &entity.MemoryCard{Title: "pinned", IsPinned: true}
	var gotPinned bool
	memory := &fakeMemoryRepository{
		setPinnedFn: func(_ context.Context, u, c uuid.UUID, pinned bool) (*entity.MemoryCard, error) {
			gotPinned = pinned
			return newCard, nil
		},
	}
	svc := NewMemoryService(memory, captures, nil, nil)

	agg, err := svc.SetPinned(context.Background(), userID, captureID, true)
	if err != nil {
		t.Fatalf("SetPinned: %v", err)
	}
	if !gotPinned {
		t.Fatal("expected pinned=true passed to repository")
	}
	if agg.MemoryCard != newCard {
		t.Fatalf("aggregate must use the card from repository")
	}
}

func TestMemoryServiceSetPinnedNotFound(t *testing.T) {
	svc := NewMemoryService(&fakeMemoryRepository{}, newMemoryCaptureRepository(), nil, nil)
	_, err := svc.SetPinned(context.Background(), uuid.New(), uuid.New(), true)
	if !errors.Is(err, ErrMemoryNotFound) {
		t.Fatalf("expected memory not found, got %v", err)
	}
}

func TestMemoryServiceArchiveIsIdempotent(t *testing.T) {
	var gotStatus string
	memory := &fakeMemoryRepository{
		setLifecycleFn: func(_ context.Context, u, c uuid.UUID, status string) (*entity.CaptureAggregate, error) {
			gotStatus = status
			return &entity.CaptureAggregate{Capture: &entity.Capture{LifecycleStatus: "archived"}}, nil
		},
	}
	svc := NewMemoryService(memory, newMemoryCaptureRepository(), nil, nil)

	agg, err := svc.Archive(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if gotStatus != "archived" {
		t.Fatalf("expected archived status, got %q", gotStatus)
	}
	if agg.Capture.LifecycleStatus != "archived" {
		t.Fatalf("expected archived aggregate, got %q", agg.Capture.LifecycleStatus)
	}
}

func TestMemoryServiceArchiveTrashedReturnsNotFound(t *testing.T) {
	memory := &fakeMemoryRepository{
		setLifecycleFn: func(context.Context, uuid.UUID, uuid.UUID, string) (*entity.CaptureAggregate, error) {
			return nil, repository.ErrMemoryNotFound
		},
	}
	svc := NewMemoryService(memory, newMemoryCaptureRepository(), nil, nil)
	_, err := svc.Archive(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrMemoryNotFound) {
		t.Fatalf("expected memory not found for trashed archive, got %v", err)
	}
}

func TestMemoryServiceDeleteTrashes(t *testing.T) {
	var gotStatus string
	memory := &fakeMemoryRepository{
		setLifecycleFn: func(_ context.Context, u, c uuid.UUID, status string) (*entity.CaptureAggregate, error) {
			gotStatus = status
			return &entity.CaptureAggregate{Capture: &entity.Capture{LifecycleStatus: "trashed"}}, nil
		},
	}
	svc := NewMemoryService(memory, newMemoryCaptureRepository(), nil, nil)

	agg, err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if gotStatus != "trashed" {
		t.Fatalf("expected trashed status, got %q", gotStatus)
	}
	if agg.Capture.LifecycleStatus != "trashed" {
		t.Fatalf("expected trashed aggregate, got %q", agg.Capture.LifecycleStatus)
	}
}

func TestMemoryServiceDeleteAlreadyTrashedReturnsNotFound(t *testing.T) {
	memory := &fakeMemoryRepository{
		setLifecycleFn: func(context.Context, uuid.UUID, uuid.UUID, string) (*entity.CaptureAggregate, error) {
			return nil, repository.ErrMemoryNotFound
		},
	}
	svc := NewMemoryService(memory, newMemoryCaptureRepository(), nil, nil)
	_, err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrMemoryNotFound) {
		t.Fatalf("expected memory not found, got %v", err)
	}
}

func TestMemoryServiceFallbackFieldsDerivedAtCreate(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	text := "这是一段用于测试的记忆内容。" + strings.Repeat("补充说明需要超过两百个字符以便验证摘要截断逻辑，这里继续填充足够多的内容让总长度明显超出上限值。", 3)
	createMemoryCapture(t, captures, userID, captureID, text)

	key := captureKey{userID, captureID}
	agg := captures.items[key]
	if agg.MemoryCard.Summary == nil {
		t.Fatal("expected fallback summary derived at create")
	}
	if want := fallbackSummary(text); *agg.MemoryCard.Summary != *want {
		t.Fatalf("summary = %q, want %q", *agg.MemoryCard.Summary, *want)
	}
	if len(agg.MemoryCard.Tags) != 0 || len(agg.MemoryCard.KeyPoints) != 0 {
		t.Fatalf("fallback card must have empty tags/key_points, got %#v/%#v", agg.MemoryCard.Tags, agg.MemoryCard.KeyPoints)
	}
	revs := captures.revisions[key]
	if len(revs) != 1 || revs[0].Source != entity.EnrichmentSourceFallback {
		t.Fatalf("expected one fallback revision, got %#v", revs)
	}
	if revs[0].CardVersion != agg.MemoryCard.Version {
		t.Fatalf("revision card_version %d must match card version %d", revs[0].CardVersion, agg.MemoryCard.Version)
	}
	if revs[0].SourceRevision != 1 {
		t.Fatalf("expected fallback source_revision 1, got %d", revs[0].SourceRevision)
	}
}

func TestMemoryServiceCorrectMapsVersionConflict(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "text")
	memory := &fakeMemoryRepository{
		appendFn: func(context.Context, uuid.UUID, uuid.UUID, *entity.EnrichmentRevision, *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
			return nil, nil, repository.ErrMemoryVersionConflict
		},
	}
	svc := NewMemoryService(memory, captures, nil, nil)
	title := "x"
	_, _, err := svc.Correct(context.Background(), userID, captureID, CorrectMemoryInput{Title: &title})
	if !errors.Is(err, ErrMemoryVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
}
