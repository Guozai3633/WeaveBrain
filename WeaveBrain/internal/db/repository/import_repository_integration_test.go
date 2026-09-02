package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fixedHash returns a 64-char SHA-256-like hex value for seeding.
func fixedHash(s string) string {
	return strings.Repeat("0", 64-len(s)) + s
}

func TestImportRepositoryPostgresIntegration(t *testing.T) {
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

	repo := NewImportRepository(NewPgConn(pool))
	jobID := uuid.New()
	row1ID := uuid.New()
	row2ID := uuid.New()
	now := time.Now().UTC()

	job := &entity.ImportJob{
		ID:            jobID,
		UserID:        userID,
		SourceName:    "旧备忘录",
		Format:        entity.ImportFormatCSV,
		OriginalFilename: strPtrRepo("notes.csv"),
		RawText:       "external_id,content\nt1,first\nt2,second",
		ColumnMapping: map[string]any{"external_id": "external_id", "content": "content"},
		Separator:     "---",
		Timezone:      strPtrRepo("Asia/Shanghai"),
		TotalRows:     2,
		ValidRows:     2,
		Status:        entity.ImportJobStatusDraft,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	rows := []*entity.ImportRow{
		{
			ID:               row1ID,
			ImportJobID:      jobID,
			UserID:           userID,
			RowNumber:        1,
			ExternalID:       strPtrRepo("t1"),
			RawPayload:       map[string]any{"external_id": "t1", "content": "first"},
			NormalizedPayload: map[string]any{"content": "first"},
			Content:          strPtrRepo("first"),
			ContentHash:      strPtrRepo(fixedHash("first")),
			ValidationErrors: []entity.ImportValidationError{},
			DedupeStatus:     entity.ImportDedupeNone,
			Status:           entity.ImportRowStatusPending,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
		{
			ID:               row2ID,
			ImportJobID:      jobID,
			UserID:           userID,
			RowNumber:        2,
			ExternalID:       strPtrRepo("t2"),
			RawPayload:       map[string]any{"external_id": "t2", "content": "second"},
			NormalizedPayload: map[string]any{"content": "second"},
			Content:          strPtrRepo("second"),
			ContentHash:      strPtrRepo(fixedHash("second")),
			ValidationErrors: []entity.ImportValidationError{},
			DedupeStatus:     entity.ImportDedupeNone,
			Status:           entity.ImportRowStatusPending,
			CreatedAt:        now,
			UpdatedAt:        now,
		},
	}

	if err := repo.CreateJob(ctx, job, rows); err != nil {
		t.Fatalf("create import job: %v", err)
	}

	var jobCount, rowCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM import_jobs WHERE user_id=$1", userID).Scan(&jobCount); err != nil {
		t.Fatalf("count import jobs: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM import_rows WHERE user_id=$1", userID).Scan(&rowCount); err != nil {
		t.Fatalf("count import rows: %v", err)
	}
	if jobCount != 1 || rowCount != 2 {
		t.Fatalf("CreateJob atomicity: jobs=%d rows=%d, want 1/2", jobCount, rowCount)
	}

	gotJob, err := repo.GetJob(ctx, userID, jobID)
	if err != nil {
		t.Fatalf("get import job: %v", err)
	}
	if gotJob.Format != entity.ImportFormatCSV || gotJob.TotalRows != 2 || gotJob.Status != entity.ImportJobStatusDraft {
		t.Fatalf("unexpected job fields: %#v", gotJob)
	}
	if _, err := repo.GetJob(ctx, otherUserID, jobID); !errors.Is(err, ErrImportNotFound) {
		t.Fatalf("other user must see import not found, got %v", err)
	}

	listed, err := repo.ListRows(ctx, userID, jobID, 1, 0)
	if err != nil {
		t.Fatalf("list import rows: %v", err)
	}
	if len(listed) != 1 || listed[0].RowNumber != 1 {
		t.Fatalf("ListRows limit=1 expected row 1 only, got %#v", listed)
	}
	if len(listed[0].RawPayload) == 0 {
		t.Fatalf("expected raw payload decoded, got %#v", listed[0].RawPayload)
	}

	byNumbers, err := repo.GetRowsByNumbers(ctx, userID, jobID, []int{2, 1})
	if err != nil {
		t.Fatalf("get import rows by numbers: %v", err)
	}
	if len(byNumbers) != 2 || byNumbers[0].RowNumber != 1 || byNumbers[1].RowNumber != 2 {
		t.Fatalf("GetRowsByNumbers expected rows 1,2 ordered, got %#v", byNumbers)
	}

	// Seed captures carrying import metadata; the first is referenced below as
	// the committed capture for row1 and used by the dedup queries.
	captureRepo := NewCaptureRepository(NewPgConn(pool))
	captureID1 := seedCapture(t, ctx, captureRepo, userID, "src", "t1", fixedHash("same-content"), "已导入的同内容")
	seedCapture(t, ctx, captureRepo, userID, "src", "t2", fixedHash("other"), "另一条")

	if err := repo.MarkRowState(ctx, userID, row1ID, entity.ImportRowStatusImported, &captureID1, &now); err != nil {
		t.Fatalf("mark row state: %v", err)
	}
	var row1Status string
	var row1CaptureID uuid.UUID
	if err := pool.QueryRow(
		ctx,
		"SELECT status, capture_id FROM import_rows WHERE id=$1",
		row1ID,
	).Scan(&row1Status, &row1CaptureID); err != nil {
		t.Fatalf("read row1 status: %v", err)
	}
	if row1Status != "imported" || row1CaptureID != captureID1 {
		t.Fatalf("MarkRowState: status=%q capture_id=%s", row1Status, row1CaptureID)
	}

	proposals := []entity.ImportFieldProposal{
		{
			ID:            uuid.New(),
			FieldName:     "primary_type",
			ProposedValue: "note",
			Provenance:    entity.ProvenanceAI,
			ApplyPolicy:   entity.ApplyPolicySafeAuto,
			Confidence:    0.9,
			EvidenceSpans: []entity.EvidenceSpan{{Start: 0, End: 5, Quote: "first"}},
			Status:        entity.ProposalStatusPending,
		},
	}
	if err := repo.SetRowCompletion(ctx, userID, row2ID, proposals); err != nil {
		t.Fatalf("set row completion: %v", err)
	}
	updatedRow2, err := repo.GetRowsByNumbers(ctx, userID, jobID, []int{2})
	if err != nil {
		t.Fatalf("get row2 after completion: %v", err)
	}
	if len(updatedRow2) != 1 || len(updatedRow2[0].CompletionProposals) != 1 {
		t.Fatalf("expected 1 completion proposal on row2, got %#v", updatedRow2)
	}
	if updatedRow2[0].CompletionProposals[0].FieldName != "primary_type" {
		t.Fatalf("unexpected proposal: %#v", updatedRow2[0].CompletionProposals[0])
	}

	row2 := updatedRow2[0]
	row2.DedupeStatus = entity.ImportDedupeDuplicateExternal
	row2.Status = entity.ImportRowStatusSkipped
	if err := repo.UpdateRow(ctx, row2); err != nil {
		t.Fatalf("update row: %v", err)
	}
	afterUpdate, err := repo.GetRowsByNumbers(ctx, userID, jobID, []int{2})
	if err != nil {
		t.Fatalf("get row2 after update: %v", err)
	}
	if afterUpdate[0].Status != entity.ImportRowStatusSkipped || afterUpdate[0].DedupeStatus != entity.ImportDedupeDuplicateExternal {
		t.Fatalf("UpdateRow did not persist status/dedupe: %#v", afterUpdate[0])
	}

	// UpdateJob status + counts.
	job.Status = entity.ImportJobStatusCompleted
	job.ImportedRows = 1
	job.SkippedRows = 1
	committedAt := now.Add(time.Minute)
	job.CommittedAt = &committedAt
	if err := repo.UpdateJob(ctx, job); err != nil {
		t.Fatalf("update job: %v", err)
	}
	finalJob, err := repo.GetJob(ctx, userID, jobID)
	if err != nil {
		t.Fatalf("get final job: %v", err)
	}
	if finalJob.Status != entity.ImportJobStatusCompleted || finalJob.ImportedRows != 1 || finalJob.CommittedAt == nil {
		t.Fatalf("UpdateJob did not persist: %#v", finalJob)
	}

	// Unique (job_id, row_number) guard.
	dupRow := rows[0]
	dupRow.ID = uuid.New()
	if err := repo.CreateJob(ctx, job, []*entity.ImportRow{dupRow}); err == nil {
		t.Fatal("expected unique (import_job_id, row_number) violation")
	}

	// Dedup queries against the captures seeded above.
	extDup, err := repo.FindExternalDuplicates(ctx, userID, "src", []string{"t1", "unknown"})
	if err != nil {
		t.Fatalf("find external duplicates: %v", err)
	}
	if len(extDup) != 1 || extDup[0] != "t1" {
		t.Fatalf("expected external duplicate key [t1], got %v", extDup)
	}
	hashDup, err := repo.FindContentHashMatches(ctx, userID, []string{fixedHash("same-content"), fixedHash("nope")})
	if err != nil {
		t.Fatalf("find content hash matches: %v", err)
	}
	if len(hashDup) != 1 || hashDup[0] != fixedHash("same-content") {
		t.Fatalf("expected content hash match, got %v", hashDup)
	}
	noDup, err := repo.FindExternalDuplicates(ctx, userID, "src", []string{"t-absent"})
	if err != nil {
		t.Fatalf("find external duplicates absent: %v", err)
	}
	if len(noDup) != 0 {
		t.Fatalf("expected no external duplicate, got %v", noDup)
	}
}

func seedCapture(t *testing.T, ctx context.Context, repo CaptureRepository, userID uuid.UUID, sourceName, externalID, contentHash, text string) uuid.UUID {
	t.Helper()
	captureID := uuid.New()
	cardID := uuid.New()
	capture := &entity.Capture{
		ID:                  captureID,
		UserID:              userID,
		Kind:                entity.CaptureKindImport,
		OriginalText:        &text,
		CapturedAtPrecision: "unknown",
		Source:              "import",
		ExternalID:          &externalID,
		SourceName:          &sourceName,
		ContentHash:         &contentHash,
		PrivacyMode:         "cloud_allowed",
		RequestHash:         strings.Repeat(contentHash[:8], 8),
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
		Source:         entity.EnrichmentSourceImport,
		SourceRevision: 1,
		Changes:        map[string]any{"title": text},
		Provenance:     map[string]any{"source": "import"},
	}
	if _, err := repo.Create(ctx, capture, card, rev, entity.DefaultPolicySnapshot()); err != nil {
		t.Fatalf("seed capture %s: %v", externalID, err)
	}
	return captureID
}

func strPtrRepo(s string) *string { return &s }
