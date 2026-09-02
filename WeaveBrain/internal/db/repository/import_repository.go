package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrImportNotFound indicates an import job/row does not exist or is not owned
// by the caller.
var ErrImportNotFound = errors.New("import not found")

type importRepository struct {
	db Conn
}

// NewImportRepository creates an ImportRepository backed by the given Conn.
func NewImportRepository(db Conn) ImportRepository {
	return &importRepository{db: db}
}

func (r *importRepository) CreateJob(ctx context.Context, job *entity.ImportJob, rows []*entity.ImportRow) error {
	mappingJSON, err := json.Marshal(job.ColumnMapping)
	if err != nil {
		return fmt.Errorf("encode import column mapping: %w", err)
	}

	const jobQuery = `
		INSERT INTO import_jobs (
			id,
			user_id,
			source_name,
			format,
			original_filename,
			raw_text,
			column_mapping,
			separator,
			timezone,
			total_rows,
			valid_rows,
			invalid_rows,
			duplicate_rows,
			needs_input_rows,
			imported_rows,
			skipped_rows,
			failed_rows,
			status
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`
	if _, err := r.db.Exec(
		ctx,
		jobQuery,
		job.ID,
		job.UserID,
		job.SourceName,
		string(job.Format),
		job.OriginalFilename,
		job.RawText,
		mappingJSON,
		job.Separator,
		job.Timezone,
		job.TotalRows,
		job.ValidRows,
		job.InvalidRows,
		job.DuplicateRows,
		job.NeedsInputRows,
		job.ImportedRows,
		job.SkippedRows,
		job.FailedRows,
		string(job.Status),
	); err != nil {
		return fmt.Errorf("insert import job: %w", err)
	}

	const rowQuery = `
		INSERT INTO import_rows (
			id,
			import_job_id,
			user_id,
			row_number,
			external_id,
			raw_payload,
			normalized_payload,
			content,
			content_hash,
			validation_errors,
			dedupe_status,
			capture_id,
			status,
			completion_proposals
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8, $9, $10::jsonb, $11, $12, $13, $14::jsonb)
	`
	for _, row := range rows {
		rawJSON, err := json.Marshal(row.RawPayload)
		if err != nil {
			return fmt.Errorf("encode import row raw payload: %w", err)
		}
		normalizedJSON, err := json.Marshal(row.NormalizedPayload)
		if err != nil {
			return fmt.Errorf("encode import row normalized payload: %w", err)
		}
		errorsJSON, err := json.Marshal(row.ValidationErrors)
		if err != nil {
			return fmt.Errorf("encode import row validation errors: %w", err)
		}
		proposalsJSON, err := json.Marshal(row.CompletionProposals)
		if err != nil {
			return fmt.Errorf("encode import row completion proposals: %w", err)
		}
		if _, err := r.db.Exec(
			ctx,
			rowQuery,
			row.ID,
			row.ImportJobID,
			row.UserID,
			row.RowNumber,
			row.ExternalID,
			rawJSON,
			normalizedJSON,
			row.Content,
			row.ContentHash,
			errorsJSON,
			string(row.DedupeStatus),
			row.CaptureID,
			string(row.Status),
			proposalsJSON,
		); err != nil {
			return fmt.Errorf("insert import row %d: %w", row.RowNumber, err)
		}
	}
	return nil
}

const importJobColumns = `
	id,
	user_id,
	source_name,
	format,
	original_filename,
	raw_text,
	column_mapping,
	separator,
	timezone,
	total_rows,
	valid_rows,
	invalid_rows,
	duplicate_rows,
	needs_input_rows,
	imported_rows,
	skipped_rows,
	failed_rows,
	status,
	committed_at,
	cancelled_at,
	created_at,
	updated_at
`

func (r *importRepository) GetJob(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error) {
	const query = `
		SELECT ` + importJobColumns + `
		FROM import_jobs
		WHERE user_id = $1 AND id = $2
	`
	job := &entity.ImportJob{}
	var format, status string
	var mappingJSON []byte
	if err := r.db.QueryRow(ctx, query, userID, jobID).Scan(
		&job.ID,
		&job.UserID,
		&job.SourceName,
		&format,
		&job.OriginalFilename,
		&job.RawText,
		&mappingJSON,
		&job.Separator,
		&job.Timezone,
		&job.TotalRows,
		&job.ValidRows,
		&job.InvalidRows,
		&job.DuplicateRows,
		&job.NeedsInputRows,
		&job.ImportedRows,
		&job.SkippedRows,
		&job.FailedRows,
		&status,
		&job.CommittedAt,
		&job.CancelledAt,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrImportNotFound
		}
		return nil, fmt.Errorf("get import job: %w", err)
	}
	job.Format = entity.ImportFormat(format)
	job.Status = entity.ImportJobStatus(status)
	if err := json.Unmarshal(mappingJSON, &job.ColumnMapping); err != nil {
		return nil, fmt.Errorf("decode import column mapping: %w", err)
	}
	return job, nil
}

const importRowColumns = `
	id,
	import_job_id,
	user_id,
	row_number,
	external_id,
	raw_payload,
	normalized_payload,
	content,
	content_hash,
	validation_errors,
	dedupe_status,
	capture_id,
	status,
	completion_proposals,
	imported_at,
	created_at,
	updated_at
`

func (r *importRepository) ListRows(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, limit, offset int) ([]*entity.ImportRow, error) {
	const query = `
		SELECT ` + importRowColumns + `
		FROM import_rows
		WHERE user_id = $1 AND import_job_id = $2
		ORDER BY row_number
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.Query(ctx, query, userID, jobID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list import rows: %w", err)
	}
	defer rows.Close()

	items := make([]*entity.ImportRow, 0)
	for rows.Next() {
		row, err := scanImportRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate import rows: %w", err)
	}
	return items, nil
}

func (r *importRepository) GetRowsByNumbers(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, rowNumbers []int) ([]*entity.ImportRow, error) {
	const query = `
		SELECT ` + importRowColumns + `
		FROM import_rows
		WHERE user_id = $1 AND import_job_id = $2 AND row_number = ANY($3::int[])
		ORDER BY row_number
	`
	rows, err := r.db.Query(ctx, query, userID, jobID, rowNumbers)
	if err != nil {
		return nil, fmt.Errorf("get import rows by numbers: %w", err)
	}
	defer rows.Close()

	items := make([]*entity.ImportRow, 0)
	for rows.Next() {
		row, err := scanImportRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate import rows by numbers: %w", err)
	}
	return items, nil
}

func (r *importRepository) UpdateJob(ctx context.Context, job *entity.ImportJob) error {
	const query = `
		UPDATE import_jobs
		SET status = $3,
		    total_rows = $4,
		    valid_rows = $5,
		    invalid_rows = $6,
		    duplicate_rows = $7,
		    needs_input_rows = $8,
		    imported_rows = $9,
		    skipped_rows = $10,
		    failed_rows = $11,
		    committed_at = $12,
		    cancelled_at = $13,
		    updated_at = now()
		WHERE user_id = $1 AND id = $2
	`
	tag, err := r.db.Exec(
		ctx,
		query,
		job.UserID,
		job.ID,
		string(job.Status),
		job.TotalRows,
		job.ValidRows,
		job.InvalidRows,
		job.DuplicateRows,
		job.NeedsInputRows,
		job.ImportedRows,
		job.SkippedRows,
		job.FailedRows,
		job.CommittedAt,
		job.CancelledAt,
	)
	if err != nil {
		return fmt.Errorf("update import job: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrImportNotFound
	}
	return nil
}

func (r *importRepository) UpdateRow(ctx context.Context, row *entity.ImportRow) error {
	normalizedJSON, err := json.Marshal(row.NormalizedPayload)
	if err != nil {
		return fmt.Errorf("encode import row normalized payload: %w", err)
	}
	errorsJSON, err := json.Marshal(row.ValidationErrors)
	if err != nil {
		return fmt.Errorf("encode import row validation errors: %w", err)
	}
	const query = `
		UPDATE import_rows
		SET normalized_payload = $3::jsonb,
		    content = $4,
		    content_hash = $5,
		    validation_errors = $6::jsonb,
		    dedupe_status = $7,
		    capture_id = $8,
		    status = $9,
		    imported_at = $10,
		    updated_at = now()
		WHERE user_id = $1 AND id = $2
	`
	tag, err := r.db.Exec(
		ctx,
		query,
		row.UserID,
		row.ID,
		normalizedJSON,
		row.Content,
		row.ContentHash,
		errorsJSON,
		string(row.DedupeStatus),
		row.CaptureID,
		string(row.Status),
		row.ImportedAt,
	)
	if err != nil {
		return fmt.Errorf("update import row: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrImportNotFound
	}
	return nil
}

func (r *importRepository) SetRowCompletion(ctx context.Context, userID uuid.UUID, rowID uuid.UUID, proposals []entity.ImportFieldProposal) error {
	proposalsJSON, err := json.Marshal(proposals)
	if err != nil {
		return fmt.Errorf("encode import row completion proposals: %w", err)
	}
	const query = `
		UPDATE import_rows
		SET completion_proposals = $3::jsonb,
		    updated_at = now()
		WHERE user_id = $1 AND id = $2
	`
	tag, err := r.db.Exec(ctx, query, userID, rowID, proposalsJSON)
	if err != nil {
		return fmt.Errorf("set import row completion: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrImportNotFound
	}
	return nil
}

func (r *importRepository) MarkRowState(ctx context.Context, userID uuid.UUID, rowID uuid.UUID, status entity.ImportRowStatus, captureID *uuid.UUID, importedAt *time.Time) error {
	const query = `
		UPDATE import_rows
		SET status = $3,
		    capture_id = $4,
		    imported_at = $5,
		    updated_at = now()
		WHERE user_id = $1 AND id = $2
	`
	tag, err := r.db.Exec(ctx, query, userID, rowID, string(status), captureID, importedAt)
	if err != nil {
		return fmt.Errorf("mark import row state: %w", err)
	}
	if rowsAffected(tag) == 0 {
		return ErrImportNotFound
	}
	return nil
}

// FindExternalDuplicates returns the external_ids of non-deleted captures
// matching any of the exact (source_name, external_id) dedup keys for the user.
// Returning the matching keys (not capture ids) lets the caller mark exactly
// which import rows are hard duplicates.
func (r *importRepository) FindExternalDuplicates(ctx context.Context, userID uuid.UUID, sourceName string, externalIDs []string) ([]string, error) {
	const query = `
		SELECT external_id
		FROM captures
		WHERE user_id = $1
		  AND source_name = $2
		  AND external_id = ANY($3::text[])
		  AND deleted_at IS NULL
	`
	return r.scanStringList(ctx, query, userID, sourceName, externalIDs)
}

// FindContentHashMatches returns the normalized content hashes of non-deleted
// captures matching any of the given hashes for the user.
func (r *importRepository) FindContentHashMatches(ctx context.Context, userID uuid.UUID, hashes []string) ([]string, error) {
	const query = `
		SELECT content_hash
		FROM captures
		WHERE user_id = $1
		  AND content_hash = ANY($2::text[])
		  AND deleted_at IS NULL
	`
	return r.scanStringList(ctx, query, userID, hashes)
}

func (r *importRepository) scanStringList(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query import duplicates: %w", err)
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, fmt.Errorf("scan import duplicate key: %w", err)
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate import duplicate keys: %w", err)
	}
	return items, nil
}

// rowScanner is satisfied by both pgx.Rows and pgx.Row so a single scan helper
// covers ListRows/GetRowsByNumbers.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanImportRow(scan rowScanner) (*entity.ImportRow, error) {
	row := &entity.ImportRow{}
	var rawJSON, normalizedJSON, errorsJSON, proposalsJSON []byte
	var dedupeStatus, status string
	if err := scan.Scan(
		&row.ID,
		&row.ImportJobID,
		&row.UserID,
		&row.RowNumber,
		&row.ExternalID,
		&rawJSON,
		&normalizedJSON,
		&row.Content,
		&row.ContentHash,
		&errorsJSON,
		&dedupeStatus,
		&row.CaptureID,
		&status,
		&proposalsJSON,
		&row.ImportedAt,
		&row.CreatedAt,
		&row.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan import row: %w", err)
	}
	if err := json.Unmarshal(rawJSON, &row.RawPayload); err != nil {
		return nil, fmt.Errorf("decode import row raw payload: %w", err)
	}
	if err := json.Unmarshal(normalizedJSON, &row.NormalizedPayload); err != nil {
		return nil, fmt.Errorf("decode import row normalized payload: %w", err)
	}
	if err := json.Unmarshal(errorsJSON, &row.ValidationErrors); err != nil {
		return nil, fmt.Errorf("decode import row validation errors: %w", err)
	}
	if err := json.Unmarshal(proposalsJSON, &row.CompletionProposals); err != nil {
		return nil, fmt.Errorf("decode import row completion proposals: %w", err)
	}
	row.DedupeStatus = entity.ImportDedupeStatus(dedupeStatus)
	row.Status = entity.ImportRowStatus(status)
	return row, nil
}
