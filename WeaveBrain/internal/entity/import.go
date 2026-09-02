package entity

import (
	"time"

	"github.com/google/uuid"
)

// ImportFormat is the source format of a batch import job.
type ImportFormat string

const (
	ImportFormatPlainText ImportFormat = "plain_text"
	ImportFormatCSV       ImportFormat = "csv"
	ImportFormatJSONL     ImportFormat = "jsonl"
)

// ImportJobStatus is the lifecycle of a batch import job.
type ImportJobStatus string

const (
	ImportJobStatusDraft     ImportJobStatus = "draft"
	ImportJobStatusPreviewed ImportJobStatus = "previewed"
	ImportJobStatusCompleted ImportJobStatus = "completed"
	ImportJobStatusFailed    ImportJobStatus = "failed"
	ImportJobStatusCancelled ImportJobStatus = "cancelled"
)

// ImportRowStatus is the lifecycle of one import row.
type ImportRowStatus string

const (
	ImportRowStatusPending    ImportRowStatus = "pending"
	ImportRowStatusNeedsInput ImportRowStatus = "needs_input"
	ImportRowStatusImporting  ImportRowStatus = "importing"
	ImportRowStatusImported   ImportRowStatus = "imported"
	ImportRowStatusSkipped    ImportRowStatus = "skipped"
	ImportRowStatusFailed     ImportRowStatus = "failed"
)

// ImportDedupeStatus records the dedupe verdict computed at parse time.
// duplicate_external is a hard skip; suggested is a content-hash match the
// user decides about (import or skip).
type ImportDedupeStatus string

const (
	ImportDedupeNone              ImportDedupeStatus = "none"
	ImportDedupeDuplicateExternal ImportDedupeStatus = "duplicate_external"
	ImportDedupeSuggested         ImportDedupeStatus = "suggested"
)

// ImportValidationError is one row-level validation failure. Rows with
// validation errors never import; the error report surfaces them.
type ImportValidationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ImportJob is one batch import: a draft job plus its rows. The raw text and
// the derived column mapping are stored so a preview can be re-shown.
type ImportJob struct {
	ID               uuid.UUID      `json:"id"`
	UserID           uuid.UUID      `json:"user_id"`
	SourceName       string         `json:"source_name"`
	Format           ImportFormat   `json:"format"`
	OriginalFilename *string        `json:"original_filename,omitempty"`
	RawText          string         `json:"raw_text"`
	ColumnMapping    map[string]any `json:"column_mapping"`
	Separator        string         `json:"separator"`
	Timezone         *string        `json:"timezone,omitempty"`
	TotalRows        int            `json:"total_rows"`
	ValidRows        int            `json:"valid_rows"`
	InvalidRows      int            `json:"invalid_rows"`
	DuplicateRows    int            `json:"duplicate_rows"`
	NeedsInputRows   int            `json:"needs_input_rows"`
	ImportedRows     int            `json:"imported_rows"`
	SkippedRows      int            `json:"skipped_rows"`
	FailedRows       int            `json:"failed_rows"`
	Status           ImportJobStatus `json:"status"`
	CommittedAt      *time.Time     `json:"committed_at,omitempty"`
	CancelledAt      *time.Time     `json:"cancelled_at,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// ImportRow is one row of a batch import. raw_payload keeps the original
// field values, normalized_payload the cleaned values, and completion
// proposals the AI-suggested missing fields (accepted before commit).
type ImportRow struct {
	ID                  uuid.UUID              `json:"id"`
	ImportJobID         uuid.UUID              `json:"import_job_id"`
	UserID              uuid.UUID              `json:"user_id"`
	RowNumber           int                    `json:"row_number"`
	ExternalID          *string                `json:"external_id,omitempty"`
	RawPayload          map[string]any         `json:"raw_payload"`
	NormalizedPayload   map[string]any         `json:"normalized_payload"`
	Content             *string                `json:"content,omitempty"`
	ContentHash         *string                `json:"content_hash,omitempty"`
	ValidationErrors    []ImportValidationError `json:"validation_errors"`
	DedupeStatus        ImportDedupeStatus     `json:"dedupe_status"`
	CaptureID           *uuid.UUID             `json:"capture_id,omitempty"`
	Status              ImportRowStatus        `json:"status"`
	CompletionProposals []ImportFieldProposal  `json:"completion_proposals"`
	ImportedAt          *time.Time             `json:"imported_at,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// ImportFieldProposal is one AI-suggested missing field for an import row.
// It reuses the completion proposal semantics so the same preview/accept
// flow applies to batch imports.
type ImportFieldProposal struct {
	ID            uuid.UUID      `json:"id"`
	FieldName     string         `json:"field_name"`
	ProposedValue string         `json:"proposed_value"`
	Provenance    Provenance     `json:"provenance"`
	ApplyPolicy   ApplyPolicy    `json:"apply_policy"`
	Confidence    float64        `json:"confidence"`
	EvidenceSpans []EvidenceSpan `json:"evidence_spans"`
	Status        ProposalStatus `json:"status"`
}

// ImportPreviewResult is one page of the preview (default first 10 rows)
// plus the job's column mapping so the UI can show field mapping, errors and
// suspected duplicates before commit.
type ImportPreviewResult struct {
	Job        *ImportJob   `json:"job"`
	Rows       []*ImportRow `json:"rows"`
	NextCursor *string      `json:"next_cursor,omitempty"`
}

// ImportCompletionResult is the per-row AI completion output after a
// completion:preview call. Each row carries its generated proposals.
type ImportCompletionResult struct {
	Rows []*ImportRow `json:"rows"`
}

// ImportCommitResult summarises a commit run. A repeated commit returns the
// previously recorded counts (idempotent, no new captures).
type ImportCommitResult struct {
	Imported   int             `json:"imported"`
	Failed     int             `json:"failed"`
	Skipped    int             `json:"skipped"`
	NeedsInput int             `json:"needs_input"`
	Total      int             `json:"total"`
	JobStatus  ImportJobStatus `json:"job_status"`
	CommittedAt *time.Time     `json:"committed_at,omitempty"`
}

// ImportErrorReportEntry is one row in the downloadable error report.
type ImportErrorReportEntry struct {
	RowNumber        int                     `json:"row_number"`
	ExternalID       *string                 `json:"external_id,omitempty"`
	Status           ImportRowStatus         `json:"status"`
	DedupeStatus     ImportDedupeStatus      `json:"dedupe_status"`
	ValidationErrors []ImportValidationError `json:"validation_errors"`
}
