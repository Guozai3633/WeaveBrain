package repository

import (
	"context"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCompletionRepositoryPostgresIntegration(t *testing.T) {
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

	userID := uuid.New()
	otherUserID := uuid.New()
	if _, err := pool.Exec(
		ctx,
		"INSERT INTO users (id, display_name) VALUES ($1, 'owner'), ($2, 'other')",
		userID,
		otherUserID,
	); err != nil {
		t.Fatalf("seed users: %v", err)
	}

	// Seed a ready capture + fallback card the same way CaptureRepository.Create does.
	captureID := uuid.New()
	cardID := uuid.New()
	text := "completion repository integration"
	capture := &entity.Capture{
		ID:                  captureID,
		UserID:              userID,
		Kind:                entity.CaptureKindText,
		OriginalText:        &text,
		CapturedAtPrecision: "unknown",
		Source:              "integration_test",
		PrivacyMode:         "cloud_allowed",
		RequestHash:         strings.Repeat("d", 64),
		ClientVersion:       1,
	}
	card := &entity.MemoryCard{
		ID:               cardID,
		UserID:           userID,
		CaptureID:        captureID,
		PrimaryType:      "uncategorized",
		Title:            text,
		Summary:          &text,
		Tags:             []string{},
		KeyPoints:        []string{},
		ProcessingStatus: "ready",
		Version:          1,
	}
	rev := &entity.EnrichmentRevision{
		UserID:         userID,
		CaptureID:      captureID,
		CardVersion:    1,
		Source:         entity.EnrichmentSourceFallback,
		SourceRevision: 1,
		Changes:        map[string]any{"title": text},
		Provenance:     map[string]any{"source": "fallback"},
	}
	if _, err := NewCaptureRepository(NewPgConn(pool)).Create(ctx, capture, card, rev, entity.DefaultPolicySnapshot()); err != nil {
		t.Fatalf("seed aggregate: %v", err)
	}

	repo := NewCompletionRepository(NewPgConn(pool))

	// CreateProposals stamps user/capture and returns ids.
	previewID := uuid.New()
	confidence := 0.9
	props := []*entity.CompletionProposal{
		{
			PreviewID:     previewID,
			SourceRevision: 1,
			FieldName:      "tags",
			OriginalValue:  "[]",
			ProposedValue:  `["work","ideas"]`,
			Provenance:     entity.ProvenanceAI,
			ApplyPolicy:    entity.ApplyPolicySafeAuto,
			Confidence:     &confidence,
			RiskLevel:      "low",
			EvidenceSpans:  []entity.EvidenceSpan{{Start: 0, End: 7, Quote: "complet"}},
			Status:         entity.ProposalStatusPending,
			Provider:       "ollama",
			Model:          "qwen2.5:7b",
			ConfigVersion:  "completion-prompt-v1",
		},
		{
			PreviewID:      previewID,
			SourceRevision: 1,
			FieldName:      "key_points",
			OriginalValue:  "[]",
			ProposedValue:  `["first"]`,
			Provenance:     entity.ProvenanceAI,
			ApplyPolicy:    entity.ApplyPolicySafeAuto,
			Status:         entity.ProposalStatusPending,
		},
	}
	if err := repo.CreateProposals(ctx, userID, captureID, props); err != nil {
		t.Fatalf("CreateProposals: %v", err)
	}
	if props[0].ID == uuid.Nil || props[1].ID == uuid.Nil {
		t.Fatal("CreateProposals must assign ids")
	}
	if props[0].UserID != userID || props[0].CaptureID != captureID {
		t.Fatalf("CreateProposals must stamp user/capture, got %s/%s", props[0].UserID, props[0].CaptureID)
	}
	if props[0].CreatedAt.IsZero() || props[0].UpdatedAt.IsZero() {
		t.Fatal("CreateProposals must assign timestamps")
	}

	// ListByIDs returns both, with evidence_spans round-tripped.
	got, err := repo.ListByIDs(ctx, userID, []uuid.UUID{props[0].ID, props[1].ID})
	if err != nil {
		t.Fatalf("ListByIDs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListByIDs len=%d, want 2", len(got))
	}
	var tagsProp *entity.CompletionProposal
	for _, p := range got {
		if p.ID == props[0].ID {
			tagsProp = p
		}
		if p.ProposedValue != "" && p.EvidenceSpans != nil && len(p.EvidenceSpans) == 0 {
			t.Fatalf("proposal %s must keep evidence_spans array", p.FieldName)
		}
	}
	if tagsProp == nil {
		t.Fatal("tags proposal missing from ListByIDs")
	}
	if tagsProp.UserID != userID || tagsProp.CaptureID != captureID || tagsProp.PreviewID != previewID {
		t.Fatalf("ListByIDs lost stamping: %+v", tagsProp)
	}
	if tagsProp.SourceRevision != 1 || tagsProp.FieldName != "tags" || tagsProp.ProposedValue != `["work","ideas"]` {
		t.Fatalf("ListByIDs lost column values: %+v", tagsProp)
	}
	if tagsProp.Confidence == nil || math.Abs(*tagsProp.Confidence-confidence) > 1e-3 {
		t.Fatalf("ListByIDs lost confidence: %v (want %v)", tagsProp.Confidence, confidence)
	}
	if tagsProp.RiskLevel != "low" || tagsProp.Provider != "ollama" || tagsProp.Model != "qwen2.5:7b" || tagsProp.ConfigVersion != "completion-prompt-v1" {
		t.Fatalf("ListByIDs lost metadata: %+v", tagsProp)
	}
	if len(tagsProp.EvidenceSpans) != 1 || tagsProp.EvidenceSpans[0].Quote != "complet" || tagsProp.EvidenceSpans[0].Start != 0 || tagsProp.EvidenceSpans[0].End != 7 {
		t.Fatalf("ListByIDs lost evidence spans: %+v", tagsProp.EvidenceSpans)
	}
	if tagsProp.Status != entity.ProposalStatusPending || tagsProp.AcceptedBy != nil || tagsProp.AcceptedAt != nil {
		t.Fatalf("fresh proposal must be pending with nil accept audit: %+v", tagsProp)
	}

	// Cross-user isolation: other user cannot see them.
	other, err := repo.ListByIDs(ctx, otherUserID, []uuid.UUID{props[0].ID, props[1].ID})
	if err != nil {
		t.Fatalf("other ListByIDs: %v", err)
	}
	if len(other) != 0 {
		t.Fatalf("other user must not see proposals, got %d", len(other))
	}
	otherPending, err := repo.ListPendingByCapture(ctx, otherUserID, captureID)
	if err != nil {
		t.Fatalf("other ListPendingByCapture: %v", err)
	}
	if len(otherPending) != 0 {
		t.Fatalf("other user must not see pending proposals, got %d", len(otherPending))
	}

	// ListPendingByCapture returns both pending ordered by field_name.
	pending, err := repo.ListPendingByCapture(ctx, userID, captureID)
	if err != nil {
		t.Fatalf("ListPendingByCapture: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("ListPendingByCapture len=%d, want 2", len(pending))
	}
	if pending[0].FieldName != "key_points" || pending[1].FieldName != "tags" {
		t.Fatalf("ListPendingByCapture must order by field_name, got %s, %s", pending[0].FieldName, pending[1].FieldName)
	}

	// MarkAccepted is idempotent and only touches pending; records audit.
	acceptedID := props[1].ID // key_points
	if err := repo.MarkAccepted(ctx, userID, []uuid.UUID{acceptedID, acceptedID}); err != nil {
		t.Fatalf("MarkAccepted: %v", err)
	}
	afterAccept, err := repo.ListByIDs(ctx, userID, []uuid.UUID{acceptedID})
	if err != nil {
		t.Fatalf("after-accept ListByIDs: %v", err)
	}
	if len(afterAccept) != 1 {
		t.Fatalf("after-accept len=%d", len(afterAccept))
	}
	if afterAccept[0].Status != entity.ProposalStatusAccepted {
		t.Fatalf("accepted proposal status=%s, want accepted", afterAccept[0].Status)
	}
	if afterAccept[0].AcceptedBy == nil || *afterAccept[0].AcceptedBy != userID {
		t.Fatalf("accepted proposal must record accepted_by, got %v", afterAccept[0].AcceptedBy)
	}
	if afterAccept[0].AcceptedAt == nil || afterAccept[0].AcceptedAt.IsZero() {
		t.Fatalf("accepted proposal must record accepted_at, got %v", afterAccept[0].AcceptedAt)
	}
	if afterAccept[0].AcceptedAt.After(time.Now().Add(time.Minute)) {
		t.Fatalf("accepted_at must be near now, got %v", afterAccept[0].AcceptedAt)
	}

	// ExpireAllPending leaves accepted alone, expires the rest.
	if err := repo.ExpireAllPending(ctx, userID, captureID); err != nil {
		t.Fatalf("ExpireAllPending: %v", err)
	}
	afterExpire, err := repo.ListByIDs(ctx, userID, []uuid.UUID{props[0].ID, props[1].ID})
	if err != nil {
		t.Fatalf("after-expire ListByIDs: %v", err)
	}
	statuses := map[uuid.UUID]entity.ProposalStatus{}
	for _, p := range afterExpire {
		statuses[p.ID] = p.Status
	}
	if statuses[props[0].ID] != entity.ProposalStatusExpired {
		t.Fatalf("tags must be expired, got %s", statuses[props[0].ID])
	}
	if statuses[props[1].ID] != entity.ProposalStatusAccepted {
		t.Fatalf("accepted must survive ExpireAllPending, got %s", statuses[props[1].ID])
	}

	// Create a second batch to test MarkRejected.
	second := []*entity.CompletionProposal{
		{
			PreviewID:      uuid.New(),
			SourceRevision: 1,
			FieldName:      "tags",
			ProposedValue:  `["new"]`,
			Provenance:     entity.ProvenanceAI,
			ApplyPolicy:    entity.ApplyPolicySafeAuto,
			Status:         entity.ProposalStatusPending,
		},
	}
	if err := repo.CreateProposals(ctx, userID, captureID, second); err != nil {
		t.Fatalf("CreateProposals second batch: %v", err)
	}
	if err := repo.MarkRejected(ctx, userID, []uuid.UUID{second[0].ID}); err != nil {
		t.Fatalf("MarkRejected: %v", err)
	}
	rejected, err := repo.ListByIDs(ctx, userID, []uuid.UUID{second[0].ID})
	if err != nil {
		t.Fatalf("rejected ListByIDs: %v", err)
	}
	if rejected[0].Status != entity.ProposalStatusRejected {
		t.Fatalf("proposal status=%s, want rejected", rejected[0].Status)
	}

	// MarkExpired on an already-expired row is a no-op (guarded by status='pending').
	if err := repo.MarkExpired(ctx, userID, []uuid.UUID{props[0].ID}); err != nil {
		t.Fatalf("MarkExpired no-op: %v", err)
	}

	// Composite FK cascades on capture delete.
	if _, err := pool.Exec(ctx, "DELETE FROM captures WHERE user_id = $1 AND id = $2", userID, captureID); err != nil {
		t.Fatalf("delete capture: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(
		ctx,
		"SELECT count(*) FROM completion_proposals WHERE user_id = $1 AND capture_id = $2",
		userID,
		captureID,
	).Scan(&remaining); err != nil {
		t.Fatalf("count after cascade: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("completion_proposals must cascade on capture delete, got %d rows", remaining)
	}
}
