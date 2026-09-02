package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// newImportServiceIntegrationEnv connects to the real test database and returns
// a store, an ImportService, the scripted completion generator (so tests can
// override its result/error), and a helper that seeds a fresh user. Tests that
// exercise completion must additionally seed enabled AI settings.
func newImportServiceIntegrationEnv(t *testing.T) (
	*repository.DBStore,
	*ImportService,
	*scriptedFieldProposalGenerator,
	func() uuid.UUID,
) {
	t.Helper()
	databaseURL := os.Getenv("WEAVEBRAIN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WEAVEBRAIN_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	store := repository.NewFromPool(pool)
	gen := &scriptedFieldProposalGenerator{modelName: "test-model"}
	svc := NewImportService(store, store.Import, store.AISettings, gen)
	seedUser := func() uuid.UUID {
		userID := uuid.New()
		if _, err := pool.Exec(
			ctx,
			"INSERT INTO users (id, display_name) VALUES ($1, 'import-service-test')",
			userID,
		); err != nil {
			t.Fatalf("seed import test user: %v", err)
		}
		return userID
	}
	return store, svc, gen, seedUser
}

// seedImportCapture creates an existing import capture so dedup queries can
// find it. Reuses the capture pipeline so content_hash is normalized the same
// way batch rows are hashed.
func seedImportCapture(t *testing.T, store *repository.DBStore, userID uuid.UUID, sourceName, externalID, text string) uuid.UUID {
	t.Helper()
	captureID := uuid.New()
	svc := NewCaptureService(store.Capture, &r7SettingsSource{
		settings: &entity.UserAISettings{UserID: userID, AIMemoryEnabled: true},
	})
	if _, err := svc.Create(context.Background(), userID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindImport,
		Text:          text,
		Source:        "import",
		SourceName:    stringPtr(sourceName),
		ExternalID:    stringPtr(externalID),
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("seed import capture %q: %v", externalID, err)
	}
	return captureID
}

func seedImportAISettings(t *testing.T, store *repository.DBStore, userID uuid.UUID) {
	t.Helper()
	if err := store.AISettings.Create(context.Background(), &entity.UserAISettings{
		UserID:              userID,
		AICompletionEnabled: true,
		CloudTextAllowed:    true,
	}); err != nil {
		t.Fatalf("seed import ai settings: %v", err)
	}
}

func countImportCaptures(t *testing.T, store *repository.DBStore, userID uuid.UUID) int {
	t.Helper()
	var n int
	if err := store.Pool.QueryRow(
		context.Background(),
		"SELECT count(*) FROM captures WHERE user_id = $1",
		userID,
	).Scan(&n); err != nil {
		t.Fatalf("count captures: %v", err)
	}
	return n
}

func rowHasImportError(row *entity.ImportRow, code string) bool {
	for _, e := range row.ValidationErrors {
		if e.Code == code {
			return true
		}
	}
	return false
}

func TestImportServiceCreateJobPersistsAndMarksDedupe(t *testing.T) {
	store, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	seedImportCapture(t, store, userID, "旧备忘录", "t1", "first")

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatCSV,
		SourceName: "旧备忘录",
		Content:    "external_id,content\nt1,first\n,second\n,second\n,   \n",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	if job.Status != entity.ImportJobStatusDraft {
		t.Fatalf("expected draft job, got %s", job.Status)
	}
	if job.TotalRows != 4 || job.ValidRows != 3 || job.InvalidRows != 1 ||
		job.NeedsInputRows != 1 || job.DuplicateRows != 1 {
		t.Fatalf("unexpected job counts: total=%d valid=%d invalid=%d needs_input=%d dup=%d",
			job.TotalRows, job.ValidRows, job.InvalidRows, job.NeedsInputRows, job.DuplicateRows)
	}

	rows, err := svc.ListRows(ctx, userID, job.ID, 10, 0)
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	byRow := map[int]*entity.ImportRow{}
	for _, r := range rows {
		byRow[r.RowNumber] = r
	}
	if byRow[1].DedupeStatus != entity.ImportDedupeDuplicateExternal {
		t.Fatalf("row1 expected duplicate_external, got %s", byRow[1].DedupeStatus)
	}
	if byRow[2].DedupeStatus != entity.ImportDedupeNone {
		t.Fatalf("row2 expected none, got %s", byRow[2].DedupeStatus)
	}
	if byRow[3].DedupeStatus != entity.ImportDedupeSuggested {
		t.Fatalf("row3 expected suggested, got %s", byRow[3].DedupeStatus)
	}
	if byRow[4].Status != entity.ImportRowStatusNeedsInput {
		t.Fatalf("row4 expected needs_input, got %s", byRow[4].Status)
	}
	if !rowHasImportError(byRow[4], "missing_content") {
		t.Fatalf("row4 expected missing_content error, got %#v", byRow[4].ValidationErrors)
	}
}

func TestImportServiceGetPreviewLimitsAndMarksPreviewed(t *testing.T) {
	_, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	var blocks []string
	for i := 1; i <= 12; i++ {
		blocks = append(blocks, fmt.Sprintf("第 %d 条记忆", i))
	}
	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    strings.Join(blocks, "\n---\n"),
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}

	preview, err := svc.GetPreview(ctx, userID, job.ID, 0, 0)
	if err != nil {
		t.Fatalf("get preview: %v", err)
	}
	if len(preview.Rows) != 10 {
		t.Fatalf("expected preview first 10 rows, got %d", len(preview.Rows))
	}
	if preview.NextCursor == nil || *preview.NextCursor != "10" {
		t.Fatalf("expected next_cursor 10, got %#v", preview.NextCursor)
	}
	reloaded, err := svc.GetJob(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if reloaded.Status != entity.ImportJobStatusPreviewed {
		t.Fatalf("expected job previewed after preview, got %s", reloaded.Status)
	}
}

func TestImportServiceCommitRowLevelIsolation(t *testing.T) {
	store, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	// Row 3 has blank content -> needs_input and must never block rows 1-2.
	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatCSV,
		SourceName: "旧备忘录",
		Content:    "content\n第一条\n第二条\n \n",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}

	before := countImportCaptures(t, store, userID)
	result, err := svc.Commit(ctx, userID, job.ID, ImportCommitInput{})
	if err != nil {
		t.Fatalf("commit import: %v", err)
	}
	if result.Imported != 2 || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("unexpected commit result: %#v", result)
	}
	after := countImportCaptures(t, store, userID)
	if after-before != 2 {
		t.Fatalf("expected 2 captures from clean rows, got %d new", after-before)
	}

	report, err := svc.GetErrorReport(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("get error report: %v", err)
	}
	if len(report) != 1 || report[0].RowNumber != 3 {
		t.Fatalf("expected error report with row 3 only, got %#v", report)
	}
	if report[0].Status != entity.ImportRowStatusSkipped {
		t.Fatalf("expected needs_input row skipped, got %s", report[0].Status)
	}
}

func TestImportServiceCommitIdempotentNoNewCaptures(t *testing.T) {
	store, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "第一条\n---\n第二条",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}

	first, err := svc.Commit(ctx, userID, job.ID, ImportCommitInput{})
	if err != nil {
		t.Fatalf("first commit: %v", err)
	}
	afterFirst := countImportCaptures(t, store, userID)

	second, err := svc.Commit(ctx, userID, job.ID, ImportCommitInput{})
	if err != nil {
		t.Fatalf("second commit: %v", err)
	}
	afterSecond := countImportCaptures(t, store, userID)

	if second.Imported != first.Imported || second.Skipped != first.Skipped {
		t.Fatalf("re-commit changed counts: first=%#v second=%#v", first, second)
	}
	if afterSecond != afterFirst {
		t.Fatalf("re-commit created new captures: before=%d after=%d", afterFirst, afterSecond)
	}
	if second.JobStatus != entity.ImportJobStatusCompleted {
		t.Fatalf("expected completed status on re-commit, got %s", second.JobStatus)
	}
}

func TestImportServiceCommitDuplicateExternalHardSkip(t *testing.T) {
	store, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	seedImportCapture(t, store, userID, "旧备忘录", "t1", "first")

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatCSV,
		SourceName: "旧备忘录",
		Content:    "external_id,content\nt1,first\n,second\n",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}

	before := countImportCaptures(t, store, userID) // 1 (the seeded capture)
	result, err := svc.Commit(ctx, userID, job.ID, ImportCommitInput{})
	if err != nil {
		t.Fatalf("commit import: %v", err)
	}
	if result.Imported != 1 || result.Skipped != 1 {
		t.Fatalf("expected 1 imported + 1 skipped (external dup), got %#v", result)
	}
	after := countImportCaptures(t, store, userID)
	if after-before != 1 {
		t.Fatalf("expected only the clean row captured, got %d new captures", after-before)
	}
}

func TestImportServiceCommitSuggestedImportOrSkip(t *testing.T) {
	store, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	seedImportCapture(t, store, userID, "旧备忘录", "s0", "重复内容")

	// User chooses to import the suspected duplicate.
	job1, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "重复内容",
	})
	if err != nil {
		t.Fatalf("create import job 1: %v", err)
	}
	res1, err := svc.Commit(ctx, userID, job1.ID, ImportCommitInput{DuplicateContentAction: "import"})
	if err != nil {
		t.Fatalf("commit import (import action): %v", err)
	}
	if res1.Imported != 1 {
		t.Fatalf("expected suggested row imported, got %#v", res1)
	}
	afterImport := countImportCaptures(t, store, userID) // 2 (seed + new)
	if afterImport != 2 {
		t.Fatalf("expected 2 captures after importing suggested row, got %d", afterImport)
	}

	// User chooses to skip a second suspected duplicate.
	job2, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "重复内容",
	})
	if err != nil {
		t.Fatalf("create import job 2: %v", err)
	}
	res2, err := svc.Commit(ctx, userID, job2.ID, ImportCommitInput{DuplicateContentAction: "skip"})
	if err != nil {
		t.Fatalf("commit import (skip action): %v", err)
	}
	if res2.Imported != 0 || res2.Skipped != 1 {
		t.Fatalf("expected suggested row skipped, got %#v", res2)
	}
	afterSkip := countImportCaptures(t, store, userID)
	if afterSkip != afterImport {
		t.Fatalf("skipping a suggested row must not create captures: before=%d after=%d", afterImport, afterSkip)
	}
}

func TestImportServiceCompletionDisabledWithoutConsent(t *testing.T) {
	_, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "内容",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	if _, err := svc.CompletionPreview(ctx, userID, job.ID, nil); !errors.Is(err, ErrImportCompletionDisabled) {
		t.Fatalf("expected ErrImportCompletionDisabled without consent, got %v", err)
	}
}

func TestImportServiceCompletionFailureImportsAsIs(t *testing.T) {
	store, svc, gen, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()
	seedImportAISettings(t, store, userID)

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "一条值得记住的内容",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}

	gen.err = errors.New("llm down")
	comp, err := svc.CompletionPreview(ctx, userID, job.ID, nil)
	if err != nil {
		t.Fatalf("completion preview must not fail on row error: %v", err)
	}
	if len(comp.Rows) != 1 || !rowHasImportError(comp.Rows[0], "completion_error") {
		t.Fatalf("expected completion_error on the row, got %#v", comp.Rows)
	}

	// The row must still import as-is (G7 criterion 4).
	result, err := svc.Commit(ctx, userID, job.ID, ImportCommitInput{})
	if err != nil {
		t.Fatalf("commit after completion failure: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("expected row imported as-is despite completion failure, got %#v", result)
	}
	if countImportCaptures(t, store, userID) != 1 {
		t.Fatalf("expected exactly 1 capture")
	}
}

func TestImportServiceCompletionAcceptedAppliesAiRevision(t *testing.T) {
	store, svc, gen, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()
	seedImportAISettings(t, store, userID)

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "关于工作的一项新思路",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}

	confidence := 0.9
	gen.result = []GeneratedFieldProposal{{
		FieldName:     "tags",
		ProposedValue: []string{"工作"},
		Evidence:      []string{"工作"},
		Confidence:    &confidence,
	}}
	comp, err := svc.CompletionPreview(ctx, userID, job.ID, nil)
	if err != nil {
		t.Fatalf("completion preview: %v", err)
	}
	if len(comp.Rows) != 1 || len(comp.Rows[0].CompletionProposals) != 1 {
		t.Fatalf("expected 1 completion proposal, got %#v", comp.Rows)
	}
	prop := comp.Rows[0].CompletionProposals[0]
	if prop.FieldName != "tags" || prop.Provenance != entity.ProvenanceAI {
		t.Fatalf("unexpected proposal: %#v", prop)
	}

	applied, err := svc.CompletionApply(ctx, userID, job.ID, []ImportRowSelection{{
		RowNumber:   1,
		ProposalIDs: []uuid.UUID{prop.ID},
	}})
	if err != nil {
		t.Fatalf("completion apply: %v", err)
	}
	if len(applied.Rows) != 1 || applied.Rows[0].CompletionProposals[0].Status != entity.ProposalStatusAccepted {
		t.Fatalf("expected proposal accepted, got %#v", applied.Rows)
	}

	result, err := svc.Commit(ctx, userID, job.ID, ImportCommitInput{})
	if err != nil {
		t.Fatalf("commit import: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("expected 1 imported, got %#v", result)
	}

	var aiRevisions int
	if err := store.Pool.QueryRow(
		ctx,
		"SELECT count(*) FROM memory_card_revisions WHERE user_id = $1 AND source = 'ai'",
		userID,
	).Scan(&aiRevisions); err != nil {
		t.Fatalf("count ai revisions: %v", err)
	}
	if aiRevisions != 1 {
		t.Fatalf("expected exactly 1 source=ai revision from accepted proposal, got %d", aiRevisions)
	}
}

func TestImportServiceCommitAppliesTitleTagOverrides(t *testing.T) {
	store, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatCSV,
		SourceName: "旧备忘录",
		Content:    "external_id,content,title,tags\nt9,内容九,标题九,标签A|标签B\n",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	result, err := svc.Commit(ctx, userID, job.ID, ImportCommitInput{})
	if err != nil {
		t.Fatalf("commit import: %v", err)
	}
	if result.Imported != 1 {
		t.Fatalf("expected 1 imported, got %#v", result)
	}

	rows, err := svc.ListRows(ctx, userID, job.ID, 1, 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("load rows after commit: rows=%d err=%v", len(rows), err)
	}
	var title string
	var tagsJSON []byte
	if err := store.Pool.QueryRow(
		ctx,
		"SELECT title, tags FROM memory_cards WHERE user_id = $1 AND capture_id = $2",
		userID,
		rows[0].ID,
	).Scan(&title, &tagsJSON); err != nil {
		t.Fatalf("read imported card: %v", err)
	}
	if title != "标题九" {
		t.Fatalf("expected card title override 标题九, got %q", title)
	}
	if !strings.Contains(string(tagsJSON), "标签A") || !strings.Contains(string(tagsJSON), "标签B") {
		t.Fatalf("expected tags 标签A/标签B in card, got %s", string(tagsJSON))
	}
}

func TestImportServiceCancelAndErrorReport(t *testing.T) {
	_, svc, _, seedUser := newImportServiceIntegrationEnv(t)
	userID := seedUser()
	ctx := context.Background()

	job, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "待取消的记忆",
	})
	if err != nil {
		t.Fatalf("create import job: %v", err)
	}
	cancelled, err := svc.Cancel(ctx, userID, job.ID)
	if err != nil {
		t.Fatalf("cancel import job: %v", err)
	}
	if cancelled.Status != entity.ImportJobStatusCancelled {
		t.Fatalf("expected cancelled status, got %s", cancelled.Status)
	}
	// A completed job cannot be cancelled.
	other, err := svc.CreateJob(ctx, userID, CreateImportJobInput{
		Format:     entity.ImportFormatPlainText,
		SourceName: "旧备忘录",
		Content:    "已完成",
	})
	if err != nil {
		t.Fatalf("create second job: %v", err)
	}
	if _, err := svc.Commit(ctx, userID, other.ID, ImportCommitInput{}); err != nil {
		t.Fatalf("commit second job: %v", err)
	}
	if _, err := svc.Cancel(ctx, userID, other.ID); !errors.Is(err, ErrImportAlreadyCompleted) {
		t.Fatalf("expected ErrImportAlreadyCompleted cancelling completed job, got %v", err)
	}
}
