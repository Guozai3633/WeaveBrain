package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// fakeCompletionRepository is an in-memory CompletionRepository that mirrors the
// status transitions and user scoping of the real repository.
type fakeCompletionRepository struct {
	proposals    []*entity.CompletionProposal
	createCalls  int
	expireCalls  int
	acceptedIDs  []uuid.UUID
	rejectedIDs  []uuid.UUID
	expiredIDs   []uuid.UUID
	listByIDsFn  func(context.Context, uuid.UUID, []uuid.UUID) ([]*entity.CompletionProposal, error)
}

func (f *fakeCompletionRepository) CreateProposals(
	_ context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
	proposals []*entity.CompletionProposal,
) error {
	f.createCalls++
	for _, p := range proposals {
		p.ID = uuid.New()
		p.UserID = userID
		p.CaptureID = captureID
		now := time.Now().UTC()
		p.CreatedAt = now
		p.UpdatedAt = now
		f.proposals = append(f.proposals, p)
	}
	return nil
}

func (f *fakeCompletionRepository) ListPendingByCapture(
	_ context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) ([]*entity.CompletionProposal, error) {
	var out []*entity.CompletionProposal
	for _, p := range f.proposals {
		if p.UserID == userID && p.CaptureID == captureID && p.Status == entity.ProposalStatusPending {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeCompletionRepository) ListByIDs(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) ([]*entity.CompletionProposal, error) {
	if f.listByIDsFn != nil {
		return f.listByIDsFn(ctx, userID, ids)
	}
	var out []*entity.CompletionProposal
	for _, id := range ids {
		for _, p := range f.proposals {
			if p.ID == id {
				if p.UserID == userID {
					out = append(out, p)
				}
				break
			}
		}
	}
	return out, nil
}

func (f *fakeCompletionRepository) ExpireAllPending(
	_ context.Context,
	userID uuid.UUID,
	captureID uuid.UUID,
) error {
	f.expireCalls++
	for _, p := range f.proposals {
		if p.UserID == userID && p.CaptureID == captureID && p.Status == entity.ProposalStatusPending {
			p.Status = entity.ProposalStatusExpired
		}
	}
	return nil
}

func (f *fakeCompletionRepository) MarkAccepted(_ context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	f.acceptedIDs = append(f.acceptedIDs, ids...)
	for _, p := range f.proposals {
		if p.UserID == userID && containsUUID(ids, p.ID) && p.Status == entity.ProposalStatusPending {
			p.Status = entity.ProposalStatusAccepted
			by := userID
			now := time.Now().UTC()
			p.AcceptedBy = &by
			p.AcceptedAt = &now
		}
	}
	return nil
}

func (f *fakeCompletionRepository) MarkRejected(_ context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	f.rejectedIDs = append(f.rejectedIDs, ids...)
	for _, p := range f.proposals {
		if p.UserID == userID && containsUUID(ids, p.ID) && p.Status == entity.ProposalStatusPending {
			p.Status = entity.ProposalStatusRejected
		}
	}
	return nil
}

func (f *fakeCompletionRepository) MarkExpired(_ context.Context, userID uuid.UUID, ids []uuid.UUID) error {
	f.expiredIDs = append(f.expiredIDs, ids...)
	for _, p := range f.proposals {
		if p.UserID == userID && containsUUID(ids, p.ID) && p.Status == entity.ProposalStatusPending {
			p.Status = entity.ProposalStatusExpired
		}
	}
	return nil
}

func containsUUID(list []uuid.UUID, target uuid.UUID) bool {
	for _, id := range list {
		if id == target {
			return true
		}
	}
	return false
}

// scriptedFieldProposalGenerator is a test double that returns a canned result
// or error and records whether it was called.
type scriptedFieldProposalGenerator struct {
	result    []GeneratedFieldProposal
	err       error
	calls     int
	modelName string
}

func (g *scriptedFieldProposalGenerator) GenerateFieldProposals(
	_ context.Context,
	_ string,
	_ []string,
) ([]GeneratedFieldProposal, error) {
	g.calls++
	if g.err != nil {
		return nil, g.err
	}
	return g.result, nil
}

func (g *scriptedFieldProposalGenerator) Model() string { return g.modelName }

// newCompletionServiceFixtures wires a ready capture, enabled AI settings, and
// empty fakes so each test can override what it needs.
func newCompletionServiceFixtures(t *testing.T, text string) (
	*memoryCaptureRepository,
	*fakeMemoryRepository,
	*fakeAISettingsRepo,
	*fakeCompletionRepository,
	*scriptedFieldProposalGenerator,
	*CompletionService,
	uuid.UUID,
	uuid.UUID,
) {
	t.Helper()
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, text)
	settings := &fakeAISettingsRepo{stored: &entity.UserAISettings{
		UserID:              userID,
		AICompletionEnabled: true,
		CloudTextAllowed:    true,
		Revision:            1,
	}}
	proposals := &fakeCompletionRepository{}
	gen := &scriptedFieldProposalGenerator{modelName: "qwen2.5:7b"}
	memory := &fakeMemoryRepository{}
	svc := NewCompletionService(captures, memory, settings, proposals, gen)
	return captures, memory, settings, proposals, gen, svc, userID, captureID
}

func TestCompletionServicePreviewDisabled(t *testing.T) {
	captures := newMemoryCaptureRepository()
	userID, captureID := uuid.New(), uuid.New()
	createMemoryCapture(t, captures, userID, captureID, "text")
	gen := &scriptedFieldProposalGenerator{}
	settings := &fakeAISettingsRepo{stored: &entity.UserAISettings{UserID: userID, Revision: 1}}
	svc := NewCompletionService(captures, &fakeMemoryRepository{}, settings, &fakeCompletionRepository{}, gen)

	_, err := svc.Preview(context.Background(), userID, captureID)
	if !errors.Is(err, ErrCompletionDisabled) {
		t.Fatalf("expected completion disabled, got %v", err)
	}
	if gen.calls != 0 {
		t.Fatal("generator must not be called when completion is disabled")
	}

	// Cloud text upload disabled must also gate preview (text goes to the LLM).
	settings.stored.AICompletionEnabled = true
	if _, err := svc.Preview(context.Background(), userID, captureID); !errors.Is(err, ErrCompletionDisabled) {
		t.Fatalf("expected completion disabled when cloud text upload is off, got %v", err)
	}
	if gen.calls != 0 {
		t.Fatal("generator must not be called when cloud text upload is disabled")
	}
}

func TestCompletionServicePreviewNotFoundAndNotReady(t *testing.T) {
	captures, _, _, proposals, gen, svc, userID, captureID := newCompletionServiceFixtures(t, "text")

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Preview(context.Background(), userID, uuid.New())
		if !errors.Is(err, ErrMemoryNotFound) {
			t.Fatalf("expected memory not found, got %v", err)
		}
	})

	t.Run("not ready", func(t *testing.T) {
		captures.items[captureKey{userID, captureID}].MemoryCard.ProcessingStatus = "processing"
		_, err := svc.Preview(context.Background(), userID, captureID)
		if !errors.Is(err, ErrCompletionNotReady) {
			t.Fatalf("expected not ready, got %v", err)
		}
		if gen.calls != 0 {
			t.Fatal("generator must not be called when the card is not ready")
		}
		if proposals.expireCalls != 0 {
			t.Fatal("must not expire proposals for a not-ready capture")
		}
	})
}

func TestCompletionServicePreviewGeneratesAndPersistsWithoutMutatingCard(t *testing.T) {
	captures, _, _, proposals, gen, svc, userID, captureID := newCompletionServiceFixtures(t, "今天跑完步，突然冒出两个想法：一是把灵感即时保存，二是定期整理成长期记忆。")

	confidence := 0.9
	gen.result = []GeneratedFieldProposal{
		{FieldName: "tags", ProposedValue: []string{"灵感", "记忆"}, Evidence: []string{"灵感"}, Confidence: &confidence},
		{FieldName: "key_points", ProposedValue: []string{"灵感即时保存"}, Evidence: []string{"把灵感即时保存"}, Confidence: &confidence},
		{FieldName: "primary_type", ProposedValue: "idea", Evidence: []string{"想法"}, Confidence: &confidence},
	}

	before := *captures.items[captureKey{userID, captureID}].MemoryCard
	result, err := svc.Preview(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	if gen.calls != 1 {
		t.Fatalf("expected 1 generator call, got %d", gen.calls)
	}
	if proposals.createCalls != 1 {
		t.Fatalf("expected proposals persisted, got %d creates", proposals.createCalls)
	}
	if len(result.MissingFields) != 3 {
		t.Fatalf("expected 3 missing fields (primary_type/tags/key_points), got %v", result.MissingFields)
	}
	if result.SourceRevision != before.Version {
		t.Fatalf("source_revision must equal card version, got %d want %d", result.SourceRevision, before.Version)
	}
	if len(result.Proposals) != 3 {
		t.Fatalf("expected 3 proposals, got %d", len(result.Proposals))
	}

	// Fallback card fills title/summary and marks primary_type uncategorized.
	var tags, keyPoints, primaryType *entity.CompletionProposal
	for _, p := range result.Proposals {
		switch p.FieldName {
		case "tags":
			tags = p
		case "key_points":
			keyPoints = p
		case "primary_type":
			primaryType = p
		}
		if p.Status != entity.ProposalStatusPending {
			t.Fatalf("proposal %s must be pending", p.FieldName)
		}
		if p.ApplyPolicy != entity.ApplyPolicySafeAuto {
			t.Fatalf("proposal %s with evidence must be safe_auto, got %s", p.FieldName, p.ApplyPolicy)
		}
		if p.RiskLevel != "low" {
			t.Fatalf("proposal %s with evidence must have low risk, got %s", p.FieldName, p.RiskLevel)
		}
		if len(p.EvidenceSpans) == 0 {
			t.Fatalf("proposal %s must carry evidence spans", p.FieldName)
		}
		if p.Provider != completionProvider || p.Model != "qwen2.5:7b" || p.ConfigVersion != completionConfigVersion {
			t.Fatalf("proposal %s must carry provider/model/config_version, got %+v", p.FieldName, p)
		}
		if p.SourceRevision != before.Version {
			t.Fatalf("proposal %s source_revision must equal card version", p.FieldName)
		}
	}
	if tags == nil || tags.ProposedValue != `["灵感","记忆"]` {
		t.Fatalf("tags proposal wrong: %+v", tags)
	}
	if keyPoints == nil || keyPoints.ProposedValue != `["灵感即时保存"]` {
		t.Fatalf("key_points proposal wrong: %+v", keyPoints)
	}
	if primaryType == nil || primaryType.ProposedValue != "idea" {
		t.Fatalf("primary_type proposal wrong: %+v", primaryType)
	}

	// Preview must not mutate the capture or card.
	after := *captures.items[captureKey{userID, captureID}].MemoryCard
	if after.Title != before.Title || after.PrimaryType != before.PrimaryType {
		t.Fatalf("preview must not mutate the card, got %+v", after)
	}
	if len(after.Tags) != 0 || len(after.KeyPoints) != 0 {
		t.Fatalf("preview must not fill fields, got tags=%v key_points=%v", after.Tags, after.KeyPoints)
	}
}

func TestCompletionServicePreviewExpiresStaleAndHandlesEmpty(t *testing.T) {
	captures, _, _, proposals, gen, svc, userID, captureID := newCompletionServiceFixtures(t, "text")

	t.Run("expires stale pending before generating", func(t *testing.T) {
		gen.result = []GeneratedFieldProposal{{FieldName: "tags", ProposedValue: []string{"x"}, Evidence: []string{"text"}}}
		if _, err := svc.Preview(context.Background(), userID, captureID); err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if proposals.expireCalls != 1 {
			t.Fatalf("expected ExpireAllPending to be called, got %d", proposals.expireCalls)
		}
	})

	t.Run("no missing fields returns empty without LLM", func(t *testing.T) {
		key := captureKey{userID, captureID}
		captures.items[key].MemoryCard.PrimaryType = "idea"
		captures.items[key].MemoryCard.Tags = []string{"x"}
		captures.items[key].MemoryCard.KeyPoints = []string{"y"}
		beforeCalls := gen.calls
		result, err := svc.Preview(context.Background(), userID, captureID)
		if err != nil {
			t.Fatalf("Preview: %v", err)
		}
		if gen.calls != beforeCalls {
			t.Fatal("generator must not be called when nothing is missing")
		}
		if len(result.MissingFields) != 0 || len(result.Proposals) != 0 {
			t.Fatalf("expected empty result, got %+v", result)
		}
	})
}

func TestCompletionServicePreviewLLMError(t *testing.T) {
	_, _, _, _, gen, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	gen.err = errors.New("llm exploded")
	_, err := svc.Preview(context.Background(), userID, captureID)
	if !errors.Is(err, ErrCompletionLLM) {
		t.Fatalf("expected completion LLM error, got %v", err)
	}
}

func TestCompletionServicePreviewEmptyOriginalTextReturnsEmpty(t *testing.T) {
	captures, _, _, _, gen, svc, userID, captureID := newCompletionServiceFixtures(t, "some text")
	// Force an empty original text on the stored capture.
	captures.items[captureKey{userID, captureID}].Capture.OriginalText = nil
	result, err := svc.Preview(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if gen.calls != 0 {
		t.Fatal("generator must not be called without original text")
	}
	if len(result.Proposals) != 0 {
		t.Fatalf("expected no proposals, got %d", len(result.Proposals))
	}
}

func TestCompletionServiceApplyDisabled(t *testing.T) {
	_, _, settings, _, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	settings.stored.AICompletionEnabled = false
	_, err := svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs: []uuid.UUID{uuid.New()},
	})
	if !errors.Is(err, ErrCompletionDisabled) {
		t.Fatalf("expected completion disabled, got %v", err)
	}
}

func TestCompletionServiceApplyFillsOnlyEmptyFields(t *testing.T) {
	_, memory, _, proposals, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")

	tagsID, keyPointsID, primaryTypeID := uuid.New(), uuid.New(), uuid.New()
	proposals.proposals = []*entity.CompletionProposal{
		{ID: tagsID, UserID: userID, CaptureID: captureID, PreviewID: uuid.New(),
			SourceRevision: 1, FieldName: "tags", ProposedValue: `["工作","灵感"]`,
			ApplyPolicy: entity.ApplyPolicySafeAuto, Status: entity.ProposalStatusPending},
		{ID: keyPointsID, UserID: userID, CaptureID: captureID, PreviewID: uuid.New(),
			SourceRevision: 1, FieldName: "key_points", ProposedValue: `["即时保存"]`,
			ApplyPolicy: entity.ApplyPolicySafeAuto, Status: entity.ProposalStatusPending},
		{ID: primaryTypeID, UserID: userID, CaptureID: captureID, PreviewID: uuid.New(),
			SourceRevision: 1, FieldName: "primary_type", ProposedValue: "idea",
			ApplyPolicy: entity.ApplyPolicySafeAuto, Status: entity.ProposalStatusPending},
	}

	var gotRev *entity.EnrichmentRevision
	var gotCard *entity.MemoryCard
	returnedRev := &entity.EnrichmentRevision{ID: 9, Revision: 2, CardVersion: 2}
	memory.appendFn = func(_ context.Context, u, c uuid.UUID, rev *entity.EnrichmentRevision, card *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
		gotRev = rev
		gotCard = card
		returnedCard := *card
		returnedCard.Version = card.Version + 1
		return returnedRev, &returnedCard, nil
	}

	result, err := svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs:    []uuid.UUID{tagsID, keyPointsID, primaryTypeID},
		SourceRevision: 1,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if gotRev == nil {
		t.Fatal("expected an AI revision to be appended")
	}
	if gotRev.Source != entity.EnrichmentSourceAI {
		t.Fatalf("expected ai source, got %q", gotRev.Source)
	}
	if gotRev.SourceRevision != 1 {
		t.Fatalf("expected capture source_revision 1, got %d", gotRev.SourceRevision)
	}
	comp, ok := gotRev.Provenance["_completion"].(map[string]any)
	if !ok {
		t.Fatalf("expected _completion provenance, got %#v", gotRev.Provenance)
	}
	if comp["preview_id"] == nil || comp["preview_id"] == "" {
		t.Fatal("expected preview_id in _completion provenance")
	}
	undo, ok := comp["undo"].(map[string]any)
	if !ok {
		t.Fatalf("expected undo map, got %#v", comp)
	}
	tagsUndo, ok := undo["tags"].([]string)
	if !ok || len(tagsUndo) != 0 {
		t.Fatalf("expected empty original for tags, got %#v", undo["tags"])
	}
	kpUndo, ok := undo["key_points"].([]string)
	if !ok || len(kpUndo) != 0 {
		t.Fatalf("expected empty original for key_points, got %#v", undo["key_points"])
	}
	if undo["primary_type"] != "uncategorized" {
		t.Fatalf("expected uncategorized original for primary_type, got %#v", undo["primary_type"])
	}

	if gotCard.Tags == nil || len(gotCard.Tags) != 2 || gotCard.Tags[0] != "工作" {
		t.Fatalf("tags not applied to card: %v", gotCard.Tags)
	}
	if gotCard.KeyPoints == nil || len(gotCard.KeyPoints) != 1 || gotCard.KeyPoints[0] != "即时保存" {
		t.Fatalf("key_points not applied: %v", gotCard.KeyPoints)
	}
	if gotCard.PrimaryType != "idea" {
		t.Fatalf("primary_type not applied: %s", gotCard.PrimaryType)
	}

	// Proposals are accepted and the result reflects the new card.
	if len(proposals.acceptedIDs) != 3 {
		t.Fatalf("expected 3 accepted proposals, got %v", proposals.acceptedIDs)
	}
	for _, p := range proposals.proposals {
		if p.Status != entity.ProposalStatusAccepted {
			t.Fatalf("proposal %s must be accepted, got %s", p.FieldName, p.Status)
		}
	}
	if result.Card == nil || result.Card.Version != 2 {
		t.Fatalf("result must use the refreshed card, got %+v", result.Card)
	}
	if result.Revision != returnedRev {
		t.Fatal("result must include the appended revision")
	}
	if len(result.AppliedProposalIDs) != 3 {
		t.Fatalf("expected 3 applied ids, got %v", result.AppliedProposalIDs)
	}
}

func TestCompletionServiceApplyIdempotentWhenAllAccepted(t *testing.T) {
	_, memory, _, proposals, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	id := uuid.New()
	proposals.proposals = []*entity.CompletionProposal{
		{ID: id, UserID: userID, CaptureID: captureID, FieldName: "tags",
			ProposedValue: `["x"]`, Status: entity.ProposalStatusAccepted},
	}
	appendCalled := false
	memory.appendFn = func(_ context.Context, _, _ uuid.UUID, _ *entity.EnrichmentRevision, _ *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) { appendCalled = true; return nil, &entity.MemoryCard{}, nil }

	result, err := svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs: []uuid.UUID{id},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if appendCalled {
		t.Fatal("no revision must be appended for an all-accepted no-op")
	}
	if len(result.AppliedProposalIDs) != 1 {
		t.Fatalf("expected the already-accepted ids echoed back, got %v", result.AppliedProposalIDs)
	}
}

func TestCompletionServiceApplyVersionConflict(t *testing.T) {
	captures, memory, _, proposals, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	// Bump the card version so the proposal (source_revision=1) is stale.
	captures.items[captureKey{userID, captureID}].MemoryCard.Version = 2

	id := uuid.New()
	proposals.proposals = []*entity.CompletionProposal{
		{ID: id, UserID: userID, CaptureID: captureID, FieldName: "tags",
			SourceRevision: 1, ProposedValue: `["x"]`, Status: entity.ProposalStatusPending},
	}
	appendCalled := false
	memory.appendFn = func(_ context.Context, _, _ uuid.UUID, _ *entity.EnrichmentRevision, _ *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) { appendCalled = true; return nil, &entity.MemoryCard{}, nil }

	_, err := svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs: []uuid.UUID{id},
	})
	if !errors.Is(err, ErrCompletionVersionConflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	if appendCalled {
		t.Fatal("no revision must be appended on a version conflict")
	}
	if len(proposals.expiredIDs) != 1 || proposals.expiredIDs[0] != id {
		t.Fatalf("expected the stale proposal expired, got %v", proposals.expiredIDs)
	}
}

func TestCompletionServiceApplyProtectsNonEmptyFields(t *testing.T) {
	captures, memory, _, proposals, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	// The user already set tags; the AI must not overwrite them.
	captures.items[captureKey{userID, captureID}].MemoryCard.Tags = []string{"用户已有"}

	tagsID := uuid.New()
	keyPointsID := uuid.New()
	proposals.proposals = []*entity.CompletionProposal{
		{ID: tagsID, UserID: userID, CaptureID: captureID, FieldName: "tags",
			SourceRevision: 1, ProposedValue: `["AI覆盖"]`, Status: entity.ProposalStatusPending},
		{ID: keyPointsID, UserID: userID, CaptureID: captureID, FieldName: "key_points",
			SourceRevision: 1, ProposedValue: `["要点"]`, Status: entity.ProposalStatusPending},
	}

	var gotCard *entity.MemoryCard
	memory.appendFn = func(_ context.Context, u, c uuid.UUID, rev *entity.EnrichmentRevision, card *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
		gotCard = card
		returnedCard := *card
		returnedCard.Version = card.Version + 1
		return &entity.EnrichmentRevision{ID: 3, Revision: 2, CardVersion: 2}, &returnedCard, nil
	}

	result, err := svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs:    []uuid.UUID{tagsID, keyPointsID},
		SourceRevision: 1,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Tags proposal rejected; key_points applied.
	if len(proposals.rejectedIDs) != 1 || proposals.rejectedIDs[0] != tagsID {
		t.Fatalf("expected tags proposal rejected, got %v", proposals.rejectedIDs)
	}
	if len(proposals.acceptedIDs) != 1 || proposals.acceptedIDs[0] != keyPointsID {
		t.Fatalf("expected key_points proposal accepted, got %v", proposals.acceptedIDs)
	}
	if len(result.AppliedProposalIDs) != 1 || result.AppliedProposalIDs[0] != keyPointsID {
		t.Fatalf("expected only key_points applied, got %v", result.AppliedProposalIDs)
	}
	if gotCard == nil {
		t.Fatal("expected card updated")
	}
	if len(gotCard.Tags) != 1 || gotCard.Tags[0] != "用户已有" {
		t.Fatalf("user tags must be preserved, got %v", gotCard.Tags)
	}
	if len(gotCard.KeyPoints) != 1 || gotCard.KeyPoints[0] != "要点" {
		t.Fatalf("key_points must be applied, got %v", gotCard.KeyPoints)
	}
}

func TestCompletionServiceApplyMixedPendingAndAccepted(t *testing.T) {
	_, memory, _, proposals, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	acceptedID := uuid.New()
	pendingID := uuid.New()
	proposals.proposals = []*entity.CompletionProposal{
		{ID: acceptedID, UserID: userID, CaptureID: captureID, FieldName: "tags",
			SourceRevision: 1, ProposedValue: `["旧"]`, Status: entity.ProposalStatusAccepted},
		{ID: pendingID, UserID: userID, CaptureID: captureID, FieldName: "key_points",
			SourceRevision: 1, ProposedValue: `["新"]`, Status: entity.ProposalStatusPending},
	}
	var appended []*entity.EnrichmentRevision
	memory.appendFn = func(_ context.Context, u, c uuid.UUID, rev *entity.EnrichmentRevision, card *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
		appended = append(appended, rev)
		returnedCard := *card
		returnedCard.Version = card.Version + 1
		return &entity.EnrichmentRevision{ID: 7, Revision: 2, CardVersion: 2}, &returnedCard, nil
	}

	result, err := svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs:    []uuid.UUID{acceptedID, pendingID},
		SourceRevision: 1,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(appended) != 1 {
		t.Fatalf("expected exactly one revision, got %d", len(appended))
	}
	if _, ok := appended[0].Changes["key_points"]; !ok {
		t.Fatalf("expected only key_points in changes, got %#v", appended[0].Changes)
	}
	if len(result.AppliedProposalIDs) != 1 || result.AppliedProposalIDs[0] != pendingID {
		t.Fatalf("expected only the pending proposal applied, got %v", result.AppliedProposalIDs)
	}
}

func TestCompletionServiceApplyInvalidIDs(t *testing.T) {
	_, _, _, proposals, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	// No stored proposals → ListByIDs returns fewer than requested.
	_, err := svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs: []uuid.UUID{uuid.New()},
	})
	if !errors.Is(err, ErrInvalidCompletionInput) {
		t.Fatalf("expected invalid input for missing proposal ids, got %v", err)
	}

	// Cross-capture proposal ids are rejected.
	id := uuid.New()
	otherCapture := uuid.New()
	proposals.proposals = []*entity.CompletionProposal{
		{ID: id, UserID: userID, CaptureID: otherCapture, FieldName: "tags",
			ProposedValue: `["x"]`, Status: entity.ProposalStatusPending},
	}
	_, err = svc.Apply(context.Background(), userID, captureID, ApplyCompletionInput{
		ProposalIDs: []uuid.UUID{id},
	})
	if !errors.Is(err, ErrInvalidCompletionInput) {
		t.Fatalf("expected invalid input for cross-capture ids, got %v", err)
	}
}

func TestCompletionServiceUndoNothingToUndo(t *testing.T) {
	_, memory, _, _, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	memory.listRevisionsFn = func(context.Context, uuid.UUID, uuid.UUID) ([]*entity.EnrichmentRevision, error) {
		return []*entity.EnrichmentRevision{
			{Source: entity.EnrichmentSourceFallback, Changes: map[string]any{"title": "text"}},
		}, nil
	}
	_, err := svc.Undo(context.Background(), userID, captureID)
	if !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("expected nothing to undo, got %v", err)
	}
}

func TestCompletionServiceUndoRevertsUneditedFields(t *testing.T) {
	captures, memory, _, _, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	key := captureKey{userID, captureID}
	card := captures.items[key].MemoryCard
	card.PrimaryType = "idea"
	card.Tags = []string{"工作", "灵感"}
	card.KeyPoints = []string{"即时保存"}
	card.Version = 2

	aiRev := &entity.EnrichmentRevision{
		ID: 2, Revision: 2, CardVersion: 2, Source: entity.EnrichmentSourceAI,
		SourceRevision: 1,
		Changes: map[string]any{
			"primary_type": "idea",
			"tags":         []any{"工作", "灵感"},
			"key_points":   []any{"即时保存"},
		},
		Provenance: map[string]any{
			"_completion": map[string]any{
				"preview_id": uuid.New().String(),
				"undo": map[string]any{
					"primary_type": "uncategorized",
					"tags":         []any{},
					"key_points":   []any{},
				},
			},
		},
	}
	memory.listRevisionsFn = func(context.Context, uuid.UUID, uuid.UUID) ([]*entity.EnrichmentRevision, error) {
		return []*entity.EnrichmentRevision{
			{Source: entity.EnrichmentSourceFallback, Changes: map[string]any{"title": "text"}},
			aiRev,
		}, nil
	}

	var gotRev *entity.EnrichmentRevision
	var gotCard *entity.MemoryCard
	returnedUndoRev := &entity.EnrichmentRevision{ID: 3, Revision: 3, CardVersion: 3}
	memory.appendFn = func(_ context.Context, u, c uuid.UUID, rev *entity.EnrichmentRevision, card *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
		gotRev = rev
		gotCard = card
		returnedCard := *card
		returnedCard.Version = card.Version + 1
		return returnedUndoRev, &returnedCard, nil
	}

	result, err := svc.Undo(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}

	if gotRev.Source != entity.EnrichmentSourceUser {
		t.Fatalf("undo revision must be user source, got %q", gotRev.Source)
	}
	if gotRev.Provenance["_undo"] != true {
		t.Fatalf("expected _undo provenance marker, got %#v", gotRev.Provenance)
	}
	if gotRev.Provenance["tags"] != "undo" || gotRev.Provenance["primary_type"] != "undo" {
		t.Fatalf("expected per-field undo provenance, got %#v", gotRev.Provenance)
	}
	if gotCard.PrimaryType != "uncategorized" {
		t.Fatalf("primary_type must revert to uncategorized, got %s", gotCard.PrimaryType)
	}
	if len(gotCard.Tags) != 0 || len(gotCard.KeyPoints) != 0 {
		t.Fatalf("tags/key_points must revert to empty, got %v/%v", gotCard.Tags, gotCard.KeyPoints)
	}
	if result.Card == nil || result.Card.Version != 3 {
		t.Fatalf("result must use the reverted card, got %+v", result.Card)
	}
	if result.Revision != returnedUndoRev {
		t.Fatal("result must include the undo revision returned by the repository")
	}
}

func TestCompletionServiceUndoSkipsUserEditedFields(t *testing.T) {
	captures, memory, _, _, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	key := captureKey{userID, captureID}
	card := captures.items[key].MemoryCard
	card.PrimaryType = "idea"
	card.Tags = []string{"用户改过"} // differs from AI value
	card.KeyPoints = []string{"即时保存"}
	card.Version = 2

	aiRev := &entity.EnrichmentRevision{
		ID: 2, Revision: 2, CardVersion: 2, Source: entity.EnrichmentSourceAI, SourceRevision: 1,
		Changes: map[string]any{
			"primary_type": "idea",
			"tags":         []any{"工作", "灵感"},
			"key_points":   []any{"即时保存"},
		},
		Provenance: map[string]any{
			"_completion": map[string]any{
				"preview_id": uuid.New().String(),
				"undo": map[string]any{
					"primary_type": "uncategorized",
					"tags":         []any{},
					"key_points":   []any{},
				},
			},
		},
	}
	memory.listRevisionsFn = func(context.Context, uuid.UUID, uuid.UUID) ([]*entity.EnrichmentRevision, error) {
		return []*entity.EnrichmentRevision{aiRev}, nil
	}

	var gotRev *entity.EnrichmentRevision
	var gotCard *entity.MemoryCard
	memory.appendFn = func(_ context.Context, u, c uuid.UUID, rev *entity.EnrichmentRevision, card *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) {
		gotRev = rev
		gotCard = card
		returnedCard := *card
		returnedCard.Version = card.Version + 1
		return &entity.EnrichmentRevision{ID: 3, Revision: 3, CardVersion: 3}, &returnedCard, nil
	}

	result, err := svc.Undo(context.Background(), userID, captureID)
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, ok := gotRev.Changes["tags"]; ok {
		t.Fatalf("user-edited tags must not be reverted, changes=%#v", gotRev.Changes)
	}
	if len(gotCard.Tags) != 1 || gotCard.Tags[0] != "用户改过" {
		t.Fatalf("user-edited tags must be preserved, got %v", gotCard.Tags)
	}
	if len(gotCard.KeyPoints) != 0 {
		t.Fatalf("unedited key_points must revert to empty, got %v", gotCard.KeyPoints)
	}
	if len(result.AppliedProposalIDs) != 0 {
		t.Fatalf("undo must not report applied proposal ids, got %v", result.AppliedProposalIDs)
	}
}

func TestCompletionServiceUndoNothingWhenAllEdited(t *testing.T) {
	captures, memory, _, _, _, svc, userID, captureID := newCompletionServiceFixtures(t, "text")
	key := captureKey{userID, captureID}
	card := captures.items[key].MemoryCard
	card.PrimaryType = "reflection"
	card.Tags = []string{"用户改过"}
	card.KeyPoints = []string{"用户改过"}
	card.Version = 2

	aiRev := &entity.EnrichmentRevision{
		ID: 2, Revision: 2, CardVersion: 2, Source: entity.EnrichmentSourceAI, SourceRevision: 1,
		Changes: map[string]any{
			"primary_type": "idea",
			"tags":         []any{"工作"},
			"key_points":   []any{"即时保存"},
		},
		Provenance: map[string]any{
			"_completion": map[string]any{
				"preview_id": uuid.New().String(),
				"undo": map[string]any{
					"primary_type": "uncategorized",
					"tags":         []any{},
					"key_points":   []any{},
				},
			},
		},
	}
	memory.listRevisionsFn = func(context.Context, uuid.UUID, uuid.UUID) ([]*entity.EnrichmentRevision, error) {
		return []*entity.EnrichmentRevision{aiRev}, nil
	}
	appendCalled := false
	memory.appendFn = func(_ context.Context, _, _ uuid.UUID, _ *entity.EnrichmentRevision, _ *entity.MemoryCard) (*entity.EnrichmentRevision, *entity.MemoryCard, error) { appendCalled = true; return nil, &entity.MemoryCard{}, nil }

	_, err := svc.Undo(context.Background(), userID, captureID)
	if !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("expected nothing to undo when every field was edited, got %v", err)
	}
	if appendCalled {
		t.Fatal("no revision must be appended when nothing is reverted")
	}
}
