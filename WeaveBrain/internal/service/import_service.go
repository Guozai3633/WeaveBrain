package service

import (
	"context"
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
	ErrImportNotFound           = errors.New("import not found")
	ErrImportInvalid            = errors.New("invalid import")
	ErrImportAlreadyCompleted   = errors.New("import already completed")
	ErrImportCompletionLLM      = errors.New("import completion llm unavailable")
	ErrImportCompletionDisabled = errors.New("import completion disabled")
	ErrImportRowFailed          = errors.New("import row failed")
)

// CreateImportJobInput is the validated batch-import request. Format maps to a
// parser; "txt"/"markdown" are translated to plain_text by the handler.
type CreateImportJobInput struct {
	Format           entity.ImportFormat
	SourceName       string
	Content          string
	Separator        string
	Timezone         *string
	OriginalFilename *string
}

// ImportRowSelection selects which proposals to accept for one import row.
type ImportRowSelection struct {
	RowNumber   int
	ProposalIDs []uuid.UUID
}

// ImportCommitInput controls how commit treats suspected duplicates.
// duplicate_content_action is "import" (default) or "skip"; row_actions
// override it per row_number.
type ImportCommitInput struct {
	DuplicateContentAction string
	RowActions             map[int]string
}

// ImportService orchestrates single-import metadata through the capture
// pipeline and batch imports through the parse → completion → commit flow.
// Every batch row commits in its own short transaction so one failing row
// never blocks the others (G7 criterion 1).
type ImportService struct {
	jobs      repository.ImportRepository
	store     *repository.DBStore
	settings  repository.UserAISettingsRepository
	generator FieldProposalGenerator
}

// NewImportService creates an ImportService. generator may be nil when the LLM
// is unavailable; CompletionPreview then returns ErrImportCompletionLLM while
// the rest of the flow (parse/preview/commit as-is) still works.
func NewImportService(
	store *repository.DBStore,
	jobs repository.ImportRepository,
	settings repository.UserAISettingsRepository,
	generator FieldProposalGenerator,
) *ImportService {
	return &ImportService{
		jobs:      jobs,
		store:     store,
		settings:  settings,
		generator: generator,
	}
}

func (s *ImportService) configured() error {
	if s == nil || s.jobs == nil || s.store == nil {
		return fmt.Errorf("import service is not configured")
	}
	return nil
}

// CreateJob parses the raw text, computes per-row dedupe verdicts against the
// user's existing captures, and persists the draft job plus its rows atomically.
func (s *ImportService) CreateJob(
	ctx context.Context,
	userID uuid.UUID,
	input CreateImportJobInput,
) (*entity.ImportJob, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	normalized, err := s.normalizeCreateInput(userID, input)
	if err != nil {
		return nil, err
	}

	parser := newImportParser(normalized.Format, normalized.Separator)
	parsed, columnMapping, err := parser.Parse(ctx, normalized.Content)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrImportInvalid, err)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("%w: no rows found", ErrImportInvalid)
	}
	if len(parsed) > maxImportRows {
		return nil, fmt.Errorf("%w: exceeds %d rows", ErrImportInvalid, maxImportRows)
	}

	now := time.Now().UTC()
	job := &entity.ImportJob{
		ID:               uuid.New(),
		UserID:           userID,
		SourceName:       normalized.SourceName,
		Format:           normalized.Format,
		OriginalFilename: normalized.OriginalFilename,
		RawText:          normalized.Content,
		ColumnMapping:    columnMapping,
		Separator:        normalized.Separator,
		Timezone:         normalized.Timezone,
		Status:           entity.ImportJobStatusDraft,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	txCtx, txStore, commitFn, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin import tx: %w", err)
	}
	rows, err := s.buildImportRows(txCtx, txStore, userID, job, parsed)
	if err != nil {
		_ = commitFn(false)
		return nil, err
	}
	job.TotalRows = len(rows)
	for _, r := range rows {
		if len(r.ValidationErrors) > 0 {
			job.InvalidRows++
			job.NeedsInputRows++
		} else {
			job.ValidRows++
		}
		if r.DedupeStatus == entity.ImportDedupeDuplicateExternal {
			job.DuplicateRows++
		}
	}

	if err := txStore.Import.CreateJob(txCtx, job, rows); err != nil {
		_ = commitFn(false)
		return nil, fmt.Errorf("create import job: %w", err)
	}
	if err := commitFn(true); err != nil {
		return nil, fmt.Errorf("commit import job: %w", err)
	}
	return job, nil
}

func (s *ImportService) normalizeCreateInput(userID uuid.UUID, input CreateImportJobInput) (CreateImportJobInput, error) {
	if userID == uuid.Nil {
		return input, fmt.Errorf("%w: user_id is required", ErrImportInvalid)
	}
	input.SourceName = strings.TrimSpace(input.SourceName)
	if input.SourceName == "" {
		return input, fmt.Errorf("%w: source_name is required", ErrImportInvalid)
	}
	if utf8.RuneCountInString(input.SourceName) > 100 {
		return input, fmt.Errorf("%w: source_name exceeds 100 characters", ErrImportInvalid)
	}
	switch input.Format {
	case entity.ImportFormatPlainText, entity.ImportFormatCSV, entity.ImportFormatJSONL:
	default:
		return input, fmt.Errorf("%w: unsupported format %q", ErrImportInvalid, input.Format)
	}
	if len(input.Content) > maxImportRawBytes {
		return input, fmt.Errorf("%w: content exceeds %d bytes", ErrImportInvalid, maxImportRawBytes)
	}
	if strings.TrimSpace(input.Content) == "" {
		return input, fmt.Errorf("%w: content must not be empty", ErrImportInvalid)
	}
	if input.Separator == "" {
		input.Separator = DefaultImportSeparator
	}
	if input.Timezone != nil {
		tz := strings.TrimSpace(*input.Timezone)
		if tz == "" {
			input.Timezone = nil
		} else {
			input.Timezone = &tz
		}
	}
	if input.OriginalFilename != nil {
		fn := strings.TrimSpace(*input.OriginalFilename)
		if fn == "" {
			input.OriginalFilename = nil
		} else {
			input.OriginalFilename = &fn
		}
	}
	return input, nil
}

// buildImportRows computes per-row content hashes, dedupe verdicts (exact
// external-id hard skip + content-hash "suspected"), and statuses. The dedupe
// queries run against captures inside the job transaction.
func (s *ImportService) buildImportRows(
	ctx context.Context,
	txStore *repository.DBStore,
	userID uuid.UUID,
	job *entity.ImportJob,
	parsed []ParsedImportRow,
) ([]*entity.ImportRow, error) {
	var externalIDs, hashes []string
	extSeen := map[string]bool{}
	hashSeen := map[string]bool{}
	for _, p := range parsed {
		if p.ExternalID != nil && *p.ExternalID != "" && !extSeen[*p.ExternalID] {
			externalIDs = append(externalIDs, *p.ExternalID)
			extSeen[*p.ExternalID] = true
		}
		if h := normalizeAndHashContent(p.Content); h != "" && !hashSeen[h] {
			hashes = append(hashes, h)
			hashSeen[h] = true
		}
	}

	extDup := map[string]bool{}
	if len(externalIDs) > 0 {
		matched, err := txStore.Import.FindExternalDuplicates(ctx, userID, job.SourceName, externalIDs)
		if err != nil {
			return nil, fmt.Errorf("check external duplicates: %w", err)
		}
		for _, id := range matched {
			extDup[id] = true
		}
	}
	hashDup := map[string]bool{}
	if len(hashes) > 0 {
		matched, err := txStore.Import.FindContentHashMatches(ctx, userID, hashes)
		if err != nil {
			return nil, fmt.Errorf("check content hash duplicates: %w", err)
		}
		for _, h := range matched {
			hashDup[h] = true
		}
	}

	now := time.Now().UTC()
	batchHashSeen := map[string]bool{}
	rows := make([]*entity.ImportRow, 0, len(parsed))
	for _, p := range parsed {
		hash := normalizeAndHashContent(p.Content)
		dedupe := entity.ImportDedupeNone
		if p.ExternalID != nil && *p.ExternalID != "" && extDup[*p.ExternalID] {
			dedupe = entity.ImportDedupeDuplicateExternal
		} else if hash != "" && (hashDup[hash] || batchHashSeen[hash]) {
			dedupe = entity.ImportDedupeSuggested
		}
		if hash != "" {
			batchHashSeen[hash] = true
		}

		status := entity.ImportRowStatusPending
		if len(p.Errors) > 0 {
			status = entity.ImportRowStatusNeedsInput
		}
		var contentPtr, contentHashPtr *string
		if p.Content != "" {
			contentPtr = &p.Content
		}
		if hash != "" {
			contentHashPtr = &hash
		}
		rows = append(rows, &entity.ImportRow{
			ID:                  uuid.New(),
			ImportJobID:         job.ID,
			UserID:              userID,
			RowNumber:           p.RowNumber,
			ExternalID:          p.ExternalID,
			RawPayload:          p.RawPayload,
			NormalizedPayload:   buildNormalizedPayload(p),
			Content:             contentPtr,
			ContentHash:         contentHashPtr,
			ValidationErrors:    p.Errors,
			DedupeStatus:        dedupe,
			Status:              status,
			CompletionProposals: []entity.ImportFieldProposal{},
			CreatedAt:           now,
			UpdatedAt:           now,
		})
	}
	return rows, nil
}

func buildNormalizedPayload(p ParsedImportRow) map[string]any {
	payload := map[string]any{}
	if p.ExternalID != nil {
		payload["external_id"] = *p.ExternalID
	}
	if p.Content != "" {
		payload["content"] = p.Content
	}
	if p.Title != nil {
		payload["title"] = *p.Title
	}
	if p.CapturedAt != nil {
		payload["captured_at"] = p.CapturedAt.UTC().Format(time.RFC3339)
		payload["captured_at_precision"] = p.CapturedAtPrecision
	}
	if p.Timezone != nil {
		payload["timezone"] = *p.Timezone
	}
	if len(p.Tags) > 0 {
		payload["tags"] = p.Tags
	}
	if p.SourceURL != nil {
		payload["source_url"] = *p.SourceURL
	}
	if p.LocationName != nil {
		payload["location_name"] = *p.LocationName
	}
	if p.Latitude != nil {
		payload["latitude"] = *p.Latitude
	}
	if p.Longitude != nil {
		payload["longitude"] = *p.Longitude
	}
	if p.Activity != nil {
		payload["activity"] = *p.Activity
	}
	return payload
}

// importRowMetadata is the typed subset of a row's normalized payload that the
// commit flow needs (title/tags/captured_at/timezone). Batch rows persist these
// as JSON values rather than typed columns.
type importRowMetadata struct {
	Title               *string
	Tags                []string
	CapturedAt          *time.Time
	CapturedAtPrecision string
	Timezone            *string
}

// decodeImportRowMetadata reads the typed metadata back out of a row's
// normalized payload.
func decodeImportRowMetadata(row *entity.ImportRow) importRowMetadata {
	var meta importRowMetadata
	payload := row.NormalizedPayload
	if payload == nil {
		return meta
	}
	if v, ok := payload["title"]; ok {
		if s, ok := asString(v); ok && s != "" {
			meta.Title = &s
		}
	}
	if v, ok := payload["captured_at"]; ok {
		if s, ok := asString(v); ok && s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				meta.CapturedAt = &t
			}
		}
	}
	if v, ok := payload["captured_at_precision"]; ok {
		if s, ok := asString(v); ok {
			meta.CapturedAtPrecision = s
		}
	}
	if v, ok := payload["timezone"]; ok {
		if s, ok := asString(v); ok && s != "" {
			meta.Timezone = &s
		}
	}
	if v, ok := payload["tags"]; ok {
		meta.Tags = decodeTags(v)
	}
	return meta
}

func decodeTags(v any) []string {
	switch t := v.(type) {
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	default:
		return nil
	}
}

// GetJob returns one import job owned by the user.
func (s *ImportService) GetJob(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	job, err := s.jobs.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, mapImportRepoError(err)
	}
	return job, nil
}

// ListRows returns a page of a job's rows ordered by row_number.
func (s *ImportService) ListRows(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, limit, offset int) ([]*entity.ImportRow, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	return s.jobs.ListRows(ctx, userID, jobID, limit, offset)
}

// GetPreview returns the first rows of a job plus its column mapping, marking
// the job previewed once (idempotent). Completed jobs are returned untouched.
func (s *ImportService) GetPreview(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
	limit, offset int,
) (*entity.ImportPreviewResult, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	job, err := s.jobs.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, mapImportRepoError(err)
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > maxImportRows {
		limit = maxImportRows
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.jobs.ListRows(ctx, userID, jobID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list import preview rows: %w", err)
	}

	result := &entity.ImportPreviewResult{Job: job, Rows: rows}
	if offset+len(rows) < job.TotalRows {
		next := fmt.Sprintf("%d", offset+len(rows))
		result.NextCursor = &next
	}

	if job.Status == entity.ImportJobStatusDraft {
		job.Status = entity.ImportJobStatusPreviewed
		if err := s.jobs.UpdateJob(ctx, job); err != nil {
			return nil, fmt.Errorf("mark import job previewed: %w", err)
		}
	}
	return result, nil
}

// CompletionPreview generates AI field proposals for selected rows' missing
// fields. A single row's LLM failure is recorded as a completion_error while
// the other rows still get proposals (G7 criterion 4). Sending row content to
// the model requires AI completion AND cloud-text consent, mirroring R8.
func (s *ImportService) CompletionPreview(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
	rowNumbers []int,
) (*entity.ImportCompletionResult, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	if _, err := s.jobs.GetJob(ctx, userID, jobID); err != nil {
		return nil, mapImportRepoError(err)
	}
	effective, err := s.effectiveSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !effective.AICompletionEnabled || !effective.CloudTextAllowed {
		return nil, ErrImportCompletionDisabled
	}
	if s.generator == nil {
		return nil, ErrImportCompletionLLM
	}

	rows, err := s.loadRows(ctx, userID, jobID, rowNumbers)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if len(row.ValidationErrors) > 0 || row.Content == nil || strings.TrimSpace(*row.Content) == "" {
			continue // nothing to complete for invalid/empty rows
		}
		missing := missingCardFields(candidateCardForRow(row))
		if len(missing) == 0 {
			continue
		}
		genCtx, cancel := context.WithTimeout(ctx, completionLLMTimeout)
		generated, genErr := s.generator.GenerateFieldProposals(genCtx, *row.Content, missing)
		cancel()
		if genErr != nil {
			row.ValidationErrors = append(row.ValidationErrors, entity.ImportValidationError{
				Code:    "completion_error",
				Message: "AI 补全失败，可按原样导入",
			})
			continue
		}
		row.CompletionProposals = buildImportProposals(*row.Content, generated)
		if err := s.jobs.SetRowCompletion(ctx, userID, row.ID, row.CompletionProposals); err != nil {
			return nil, fmt.Errorf("persist import completion proposals: %w", err)
		}
	}
	return &entity.ImportCompletionResult{Rows: rows}, nil
}

// CompletionApply marks the selected proposal IDs accepted and the same row's
// other pending proposals rejected. It is idempotent: a re-apply leaves the
// already-accepted proposals untouched.
func (s *ImportService) CompletionApply(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
	selections []ImportRowSelection,
) (*entity.ImportCompletionResult, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return nil, fmt.Errorf("%w: row_selections must not be empty", ErrImportInvalid)
	}
	if _, err := s.jobs.GetJob(ctx, userID, jobID); err != nil {
		return nil, mapImportRepoError(err)
	}
	rowNumbers := make([]int, 0, len(selections))
	for _, sel := range selections {
		rowNumbers = append(rowNumbers, sel.RowNumber)
	}
	rows, err := s.jobs.GetRowsByNumbers(ctx, userID, jobID, rowNumbers)
	if err != nil {
		return nil, fmt.Errorf("load rows for completion apply: %w", err)
	}
	if len(rows) != len(selections) {
		return nil, fmt.Errorf("%w: some selected rows not found", ErrImportInvalid)
	}
	selByRow := make(map[int]ImportRowSelection, len(selections))
	for _, sel := range selections {
		selByRow[sel.RowNumber] = sel
	}
	for _, row := range rows {
		sel, ok := selByRow[row.RowNumber]
		if !ok {
			continue
		}
		want := make(map[uuid.UUID]bool, len(sel.ProposalIDs))
		for _, id := range sel.ProposalIDs {
			want[id] = true
		}
		for i := range row.CompletionProposals {
			p := &row.CompletionProposals[i]
			if want[p.ID] {
				p.Status = entity.ProposalStatusAccepted
			} else if p.Status == entity.ProposalStatusPending {
				p.Status = entity.ProposalStatusRejected
			}
		}
		if err := s.jobs.SetRowCompletion(ctx, userID, row.ID, row.CompletionProposals); err != nil {
			return nil, fmt.Errorf("persist import row completion: %w", err)
		}
	}
	return &entity.ImportCompletionResult{Rows: rows}, nil
}

// Commit imports every importable row, one per transaction. Duplicate-external
// rows and rows with validation errors are skipped; suspected content-hash
// duplicates import unless the user chose to skip them. Re-committing an
// already-completed job returns the recorded counts without new captures
// (G7 criterion 3).
func (s *ImportService) Commit(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
	input ImportCommitInput,
) (*entity.ImportCommitResult, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	job, err := s.jobs.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, mapImportRepoError(err)
	}
	if job.Status == entity.ImportJobStatusCompleted {
		return &entity.ImportCommitResult{
			Imported:    job.ImportedRows,
			Failed:      job.FailedRows,
			Skipped:     job.SkippedRows,
			NeedsInput:  job.NeedsInputRows,
			Total:       job.TotalRows,
			JobStatus:   job.Status,
			CommittedAt: job.CommittedAt,
		}, nil
	}

	rows, err := s.jobs.ListRows(ctx, userID, jobID, maxImportRows, 0)
	if err != nil {
		return nil, fmt.Errorf("list import rows for commit: %w", err)
	}

	duplicateAction := strings.TrimSpace(input.DuplicateContentAction)
	if duplicateAction == "" {
		duplicateAction = "import"
	}
	if duplicateAction != "import" && duplicateAction != "skip" {
		return nil, fmt.Errorf("%w: duplicate_content_action must be import or skip", ErrImportInvalid)
	}

	now := time.Now().UTC()
	var imported, failed, skipped int
	for _, row := range rows {
		switch row.Status {
		case entity.ImportRowStatusImported:
			imported++
			continue
		case entity.ImportRowStatusSkipped:
			skipped++
			continue
		case entity.ImportRowStatusFailed:
			failed++
			continue
		}

		// Rows with validation errors can never import (G7 criterion 1):
		// they are isolated and reported, never blocking other rows.
		if len(row.ValidationErrors) > 0 {
			if err := s.jobs.MarkRowState(ctx, userID, row.ID, entity.ImportRowStatusSkipped, nil, nil); err != nil {
				return nil, fmt.Errorf("mark invalid import row skipped: %w", err)
			}
			skipped++
			continue
		}
		if row.DedupeStatus == entity.ImportDedupeDuplicateExternal {
			if err := s.jobs.MarkRowState(ctx, userID, row.ID, entity.ImportRowStatusSkipped, nil, nil); err != nil {
				return nil, fmt.Errorf("mark duplicate import row skipped: %w", err)
			}
			skipped++
			continue
		}
		if row.DedupeStatus == entity.ImportDedupeSuggested {
			action := duplicateAction
			if a, ok := input.RowActions[row.RowNumber]; ok && (a == "import" || a == "skip") {
				action = a
			}
			if action == "skip" {
				if err := s.jobs.MarkRowState(ctx, userID, row.ID, entity.ImportRowStatusSkipped, nil, nil); err != nil {
					return nil, fmt.Errorf("mark suggested import row skipped: %w", err)
				}
				skipped++
				continue
			}
		}

		if err := s.commitRow(ctx, userID, job, row); err != nil {
			// Row-level isolation: roll back this row's tx and record failure.
			if markErr := s.jobs.MarkRowState(ctx, userID, row.ID, entity.ImportRowStatusFailed, nil, nil); markErr != nil {
				return nil, fmt.Errorf("mark import row failed (commit err %v): %w", err, markErr)
			}
			failed++
			continue
		}
		imported++
	}

	job.ImportedRows = imported
	job.SkippedRows = skipped
	job.FailedRows = failed
	job.Status = entity.ImportJobStatusCompleted
	job.CommittedAt = &now
	job.UpdatedAt = now
	if err := s.jobs.UpdateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("complete import job: %w", err)
	}
	return &entity.ImportCommitResult{
		Imported:    imported,
		Failed:      failed,
		Skipped:     skipped,
		NeedsInput:  job.NeedsInputRows,
		Total:       job.TotalRows,
		JobStatus:   job.Status,
		CommittedAt: job.CommittedAt,
	}, nil
}

// commitRow imports one row inside its own transaction: create the capture
// with a deterministic id equal to the row id (re-commit idempotency via the
// capture CTE), apply accepted AI proposals, then mark the row imported.
func (s *ImportService) commitRow(
	ctx context.Context,
	userID uuid.UUID,
	job *entity.ImportJob,
	row *entity.ImportRow,
) error {
	if row.Content == nil || strings.TrimSpace(*row.Content) == "" {
		return fmt.Errorf("%w: row has no content", ErrImportRowFailed)
	}

	txCtx, txStore, commitFn, err := s.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin import row tx: %w", err)
	}

	meta := decodeImportRowMetadata(row)
	captureSvc := NewCaptureService(txStore.Capture, importSettingsSnapshot(s.settings))
	input := CreateCaptureInput{
		ID:                  row.ID,
		Kind:                entity.CaptureKindImport,
		Text:                *row.Content,
		CapturedAt:          meta.CapturedAt,
		CapturedAtPrecision: meta.CapturedAtPrecision,
		Timezone:            meta.Timezone,
		Source:              "import",
		SourceName:          stringPtr(job.SourceName),
		ExternalID:          row.ExternalID,
		TitleOverride:       meta.Title,
		TagsOverride:        meta.Tags,
		ClientVersion:       1,
	}
	result, err := captureSvc.Create(txCtx, userID, input)
	if err != nil {
		_ = commitFn(false)
		return err
	}
	// When the capture already exists (a crashed prior commit re-ran this row),
	// the proposals were already applied in that prior transaction.
	if !result.Replayed {
		if err := applyAcceptedImportProposals(txCtx, txStore, userID, row); err != nil {
			_ = commitFn(false)
			return err
		}
	}
	now := time.Now().UTC()
	if err := txStore.Import.MarkRowState(txCtx, userID, row.ID, entity.ImportRowStatusImported, &row.ID, &now); err != nil {
		_ = commitFn(false)
		return fmt.Errorf("mark import row imported: %w", err)
	}
	if err := commitFn(true); err != nil {
		return fmt.Errorf("commit import row: %w", err)
	}
	return nil
}

// GetErrorReport lists the rows that did not import (skipped/failed/needs_input)
// with their validation errors, for the downloadable report.
func (s *ImportService) GetErrorReport(
	ctx context.Context,
	userID uuid.UUID,
	jobID uuid.UUID,
) ([]entity.ImportErrorReportEntry, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	rows, err := s.jobs.ListRows(ctx, userID, jobID, maxImportRows, 0)
	if err != nil {
		return nil, fmt.Errorf("list import rows for report: %w", err)
	}
	entries := make([]entity.ImportErrorReportEntry, 0, len(rows))
	for _, row := range rows {
		if row.Status == entity.ImportRowStatusImported {
			continue
		}
		entries = append(entries, entity.ImportErrorReportEntry{
			RowNumber:        row.RowNumber,
			ExternalID:       row.ExternalID,
			Status:           row.Status,
			DedupeStatus:     row.DedupeStatus,
			ValidationErrors: row.ValidationErrors,
		})
	}
	return entries, nil
}

// Cancel marks a non-completed job cancelled.
func (s *ImportService) Cancel(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error) {
	if err := s.configured(); err != nil {
		return nil, err
	}
	job, err := s.jobs.GetJob(ctx, userID, jobID)
	if err != nil {
		return nil, mapImportRepoError(err)
	}
	if job.Status == entity.ImportJobStatusCompleted {
		return nil, ErrImportAlreadyCompleted
	}
	now := time.Now().UTC()
	job.Status = entity.ImportJobStatusCancelled
	job.CancelledAt = &now
	job.UpdatedAt = now
	if err := s.jobs.UpdateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("cancel import job: %w", err)
	}
	return job, nil
}

func (s *ImportService) effectiveSettings(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error) {
	stored, err := s.settings.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load ai settings: %w", err)
	}
	if stored == nil {
		return entity.DefaultUserAISettings(userID), nil
	}
	return stored, nil
}

func (s *ImportService) loadRows(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, rowNumbers []int) ([]*entity.ImportRow, error) {
	if len(rowNumbers) == 0 {
		return s.jobs.ListRows(ctx, userID, jobID, maxImportRows, 0)
	}
	return s.jobs.GetRowsByNumbers(ctx, userID, jobID, rowNumbers)
}

func mapImportRepoError(err error) error {
	if errors.Is(err, repository.ErrImportNotFound) {
		return ErrImportNotFound
	}
	return err
}

// importSettingsSnapshot adapts the UserAISettingsRepository to the
// AISettingsSnapshotSource the capture pipeline needs (defaults when no row).
func importSettingsSnapshot(repo repository.UserAISettingsRepository) AISettingsSnapshotSource {
	return settingsSnapshotFunc(func(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error) {
		stored, err := repo.GetByUserID(ctx, userID)
		if err != nil {
			return nil, err
		}
		if stored == nil {
			return entity.DefaultUserAISettings(userID), nil
		}
		return stored, nil
	})
}

type settingsSnapshotFunc func(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error)

func (f settingsSnapshotFunc) Get(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error) {
	return f(ctx, userID)
}

// candidateCardForRow mirrors the MemoryCard the capture pipeline will build
// for this row, so the completion preview computes missing fields consistently.
func candidateCardForRow(row *entity.ImportRow) *entity.MemoryCard {
	meta := decodeImportRowMetadata(row)
	card := &entity.MemoryCard{PrimaryType: "uncategorized", Tags: []string{}, KeyPoints: []string{}}
	if meta.Title != nil {
		card.Title = *meta.Title
	}
	if len(meta.Tags) > 0 {
		card.Tags = meta.Tags
	}
	return card
}

// buildImportProposals converts the generator's output into row proposals,
// reusing the completion service's field serialization and apply-policy logic.
func buildImportProposals(originalText string, generated []GeneratedFieldProposal) []entity.ImportFieldProposal {
	proposals := make([]entity.ImportFieldProposal, 0, len(generated))
	for _, g := range generated {
		proposedValue, err := serializeProposedValue(g.FieldName, g.ProposedValue)
		if err != nil {
			continue
		}
		spans := buildEvidenceSpans(originalText, g.Evidence)
		policy := classifyApplyPolicy(g.FieldName, len(spans) > 0)
		var confidence float64
		if clamped := clampConfidence(g.Confidence); clamped != nil {
			confidence = *clamped
		}
		proposals = append(proposals, entity.ImportFieldProposal{
			ID:            uuid.New(),
			FieldName:     g.FieldName,
			ProposedValue: proposedValue,
			Provenance:    entity.ProvenanceAI,
			ApplyPolicy:   policy,
			Confidence:    confidence,
			EvidenceSpans: spans,
			Status:        entity.ProposalStatusPending,
		})
	}
	return proposals
}

// applyAcceptedImportProposals writes a source=ai revision for a row's accepted
// proposals inside the row's commit transaction. Fields the user already set
// (e.g. a title supplied in the CSV) are protected.
func applyAcceptedImportProposals(
	ctx context.Context,
	txStore *repository.DBStore,
	userID uuid.UUID,
	row *entity.ImportRow,
) error {
	var accepted []entity.ImportFieldProposal
	for _, p := range row.CompletionProposals {
		if p.Status == entity.ProposalStatusAccepted {
			accepted = append(accepted, p)
		}
	}
	if len(accepted) == 0 {
		return nil
	}
	agg, err := txStore.Capture.GetByID(ctx, userID, row.ID)
	if err != nil {
		return fmt.Errorf("load capture for import proposals: %w", err)
	}
	if agg == nil || agg.MemoryCard == nil {
		return nil
	}
	card := agg.MemoryCard
	updated := copyMemoryCard(card)
	changes := map[string]any{}
	for _, p := range accepted {
		if !fieldEmpty(card, p.FieldName) {
			continue
		}
		value, derr := decodeProposedValue(p.FieldName, p.ProposedValue)
		if derr != nil {
			continue
		}
		applyFieldValue(updated, p.FieldName, value)
		changes[p.FieldName] = value
	}
	if len(changes) == 0 {
		return nil
	}
	var captureVersion int64 = 1
	if agg.Capture != nil {
		captureVersion = agg.Capture.Version
	}
	rev := &entity.EnrichmentRevision{
		UserID:         userID,
		CaptureID:      row.ID,
		Source:         entity.EnrichmentSourceAI,
		SourceRevision: captureVersion,
		Changes:        changes,
		Provenance:     map[string]any{"source": "ai", "_import": true},
	}
	for field := range changes {
		rev.Provenance[field] = "completion"
	}
	if _, _, err := txStore.Memory.AppendRevisionAndUpdateCard(ctx, userID, row.ID, rev, updated); err != nil {
		return fmt.Errorf("apply import completion proposals: %w", err)
	}
	return nil
}

func stringPtr(s string) *string { return &s }
