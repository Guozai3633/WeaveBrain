package repository

import (
	"context"
	"time"

	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

// UserRepository defines the interface for user persistence operations.
type UserRepository interface {
	Create(ctx context.Context, u *entity.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.User, error)
	GetByDisplayName(ctx context.Context, name string) (*entity.User, error)
	Update(ctx context.Context, u *entity.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, page, limit int) ([]*entity.User, int64, error)
}

// IdentityRepository defines the interface for authentication identity persistence.
type IdentityRepository interface {
	Create(ctx context.Context, i *entity.Identity) error
	GetByID(ctx context.Context, id int64) (*entity.Identity, error)
	GetByProvider(ctx context.Context, provider, providerID string) (*entity.Identity, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Identity, error)
	Delete(ctx context.Context, id int64) error
}

// ProjectRepository defines the interface for project persistence operations.
type ProjectRepository interface {
	Create(ctx context.Context, p *entity.Project) error
	GetByID(ctx context.Context, id int64) (*entity.Project, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]*entity.Project, error)
	Update(ctx context.Context, p *entity.Project) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.Project, int64, error)
}

// IdeaRepository defines the interface for idea persistence operations.
type IdeaRepository interface {
	Create(ctx context.Context, i *entity.Idea) error
	GetByID(ctx context.Context, id int64) (*entity.Idea, error)
	GetByProjectID(ctx context.Context, userID uuid.UUID, projectID int64, page, limit int) ([]*entity.Idea, int64, error)
	Search(ctx context.Context, userID uuid.UUID, projectID int64, query string, tags []string, limit, offset int) ([]*entity.Idea, int64, error)
	Update(ctx context.Context, i *entity.Idea) error
	Delete(ctx context.Context, id int64) error
	ListByTags(ctx context.Context, tags []string, projectID int64) ([]*entity.Idea, error)
	SearchBySimilarity(ctx context.Context, projectID int64, embedding []float32, limit int) ([]*entity.Idea, error)
	SearchGlobalBySimilarity(ctx context.Context, userID uuid.UUID, embedding []float32, threshold float64, limit int) ([]*entity.Idea, error)
	UpdateEmbedding(ctx context.Context, ideaID int64, embedding []float32) error
	GetWithoutEmbedding(ctx context.Context, limit int) ([]*entity.Idea, error)
}

// CaptureRepository persists the raw Capture, fallback card, and Outbox atomically.
type CaptureRepository interface {
	// Create atomically inserts the Capture, its fallback MemoryCard, the
	// initial fallback EnrichmentRevision, and the capture.created Outbox
	// event in one statement. The policy snapshot records the user's AI
	// consent switches at creation time so the worker later enriches according
	// to that snapshot, never a later settings change.
	Create(
		ctx context.Context,
		capture *entity.Capture,
		card *entity.MemoryCard,
		rev *entity.EnrichmentRevision,
		policy *entity.PolicySnapshot,
	) (replayed bool, err error)
	GetByID(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.CaptureAggregate, error)
	// UpdateCardTitle updates the fallback MemoryCard title for a Capture.
	// Used when an STT/user transcript becomes available for an audio Capture.
	UpdateCardTitle(ctx context.Context, userID uuid.UUID, captureID uuid.UUID, title string) error
	// FindExternalDuplicate returns the id of a non-deleted capture sharing the
	// exact (user_id, source_name, external_id) import dedup key, excluding the
	// given capture (an idempotent retry of the same capture is not a duplicate).
	// Returns nil when no other capture matches.
	FindExternalDuplicate(
		ctx context.Context,
		userID uuid.UUID,
		sourceName, externalID string,
		excludeCaptureID uuid.UUID,
	) (*uuid.UUID, error)
	// FindContentHashMatch returns the id of the first non-deleted capture whose
	// normalized content hash equals the given hash, excluding the given
	// capture. Returns nil when no other capture matches.
	FindContentHashMatch(
		ctx context.Context,
		userID uuid.UUID,
		contentHash string,
		excludeCaptureID uuid.UUID,
	) (*uuid.UUID, error)
}

// MemoryRepository persists the memory stream and its revision trail.
type MemoryRepository interface {
	// List returns one page of the memory stream (reverse-chronological by
	// creation, pinned first) matching the query's filters and cursor.
	List(ctx context.Context, userID uuid.UUID, q entity.MemoryListQuery) (*entity.MemoryListResult, error)
	// ListRevisions returns a Capture's enrichment revisions newest first.
	ListRevisions(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) ([]*entity.EnrichmentRevision, error)
	// AppendRevisionAndUpdateCard applies a user/ai revision and the card's
	// new field values atomically: revision = max+1, card version +1. It
	// returns the inserted revision and the refreshed card.
	AppendRevisionAndUpdateCard(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
		rev *entity.EnrichmentRevision,
		card *entity.MemoryCard,
	) (*entity.EnrichmentRevision, *entity.MemoryCard, error)
	// AppendNoteAndBump records a note revision (续写) and bumps the card
	// version without changing card fields. It returns the revision + card.
	AppendNoteAndBump(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
		rev *entity.EnrichmentRevision,
	) (*entity.EnrichmentRevision, *entity.MemoryCard, error)
	// SetPinned toggles the pinned flag (and pinned_at) on a memory card.
	SetPinned(ctx context.Context, userID uuid.UUID, captureID uuid.UUID, pinned bool) (*entity.MemoryCard, error)
	// SetLifecycle moves a Capture to the given lifecycle status (archived /
	// trashed / deleted). trashed and deleted also set deleted_at. It returns
	// the refreshed capture aggregate.
	SetLifecycle(ctx context.Context, userID uuid.UUID, captureID uuid.UUID, status string) (*entity.CaptureAggregate, error)
}

// AudioAssetRepository manages chunked, checksum-verified audio uploads.
type AudioAssetRepository interface {
	// Initiate creates the asset row, idempotent on (user_id, id).
	Initiate(ctx context.Context, asset *entity.AudioAsset) error
	GetByID(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) (*entity.AudioAsset, error)
	GetByCaptureID(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.AudioAsset, error)
	// UpdateChunkProgress records the number of successfully received chunks.
	UpdateChunkProgress(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, receivedChunks int32) error
	// Complete finalizes a successful upload with its full-file SHA-256.
	Complete(ctx context.Context, userID uuid.UUID, assetID uuid.UUID, sha256 string) error
	// MarkFailed marks an upload as failed.
	MarkFailed(ctx context.Context, userID uuid.UUID, assetID uuid.UUID) error
}

// TranscriptRepository persists immutable transcript revisions per Capture.
type TranscriptRepository interface {
	// Append inserts the next revision for a Capture (revision = max+1, atomic).
	Append(ctx context.Context, revision *entity.TranscriptRevision) error
	GetLatest(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.TranscriptRevision, error)
	ListByCapture(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) ([]*entity.TranscriptRevision, error)
}

// UserProfileRepository defines the interface for user profile persistence.
type UserProfileRepository interface {
	Create(ctx context.Context, p *entity.UserProfile) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.UserProfile, error)
	Update(ctx context.Context, p *entity.UserProfile) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

// ReminderRepository defines the interface for reminder persistence.
type ReminderRepository interface {
	Create(ctx context.Context, rem *entity.Reminder) error
	GetByID(ctx context.Context, id int64) (*entity.Reminder, error)
	// GetPending returns system-wide pending reminders due before the given
	// time. Used by the Temporal cron for reminder delivery across all users.
	GetPending(ctx context.Context, before time.Time, limit int) ([]*entity.Reminder, error)
	// GetPendingByUser returns a single user's pending reminders due before the
	// given time. Used by the HTTP API so one user cannot read another's
	// reminders.
	GetPendingByUser(ctx context.Context, userID uuid.UUID, before time.Time, limit int) ([]*entity.Reminder, error)
	Update(ctx context.Context, rem *entity.Reminder) error
	Delete(ctx context.Context, id int64) error
}

// WorkflowRunRepository defines the interface for workflow run persistence.
type WorkflowRunRepository interface {
	Create(ctx context.Context, wr *entity.WorkflowRun) error
	GetByWorkflowID(ctx context.Context, workflowID string) (*entity.WorkflowRun, error)
	UpdateStatus(ctx context.Context, workflowID string, status string, output map[string]any, errMsg *string) error
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.WorkflowRun, int64, error)
}

// MCPAuditLogRepository defines the interface for audit log persistence.
type MCPAuditLogRepository interface {
	Create(ctx context.Context, log *entity.MCPAuditLog) error
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]*entity.MCPAuditLog, int64, error)
}

// UserMcpConfigRepository defines the interface for MCP configuration persistence.
type UserMcpConfigRepository interface {
	Upsert(ctx context.Context, config *entity.UserMcpConfig) error
	GetByNamespace(ctx context.Context, userID uuid.UUID, namespace string) (*entity.UserMcpConfig, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*entity.UserMcpConfig, error)
	Delete(ctx context.Context, userID uuid.UUID, namespace string) error
}

// UserAISettingsRepository persists per-user AI consent switches with
// optimistic-concurrency revision tracking.
type UserAISettingsRepository interface {
	// GetByUserID returns the settings row, or nil if the user has no row yet.
	GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.UserAISettings, error)
	// Create inserts a fresh row at revision 1 (the first write from the
	// implicit revision-0 default). A concurrent create for the same user
	// returns ErrAISettingsVersionConflict.
	Create(ctx context.Context, settings *entity.UserAISettings) error
	// Update applies a settings row guarded by the expected revision
	// (optimistic concurrency). It returns the new revision on success.
	Update(ctx context.Context, settings *entity.UserAISettings, expectedRevision int64) (int64, error)
}

// UserEchoSettingsRepository persists per-user echo switches (enabled +
// cadence) with optimistic-concurrency revision tracking, mirroring the AI
// settings pattern. Echo is off by default (the user opts in).
type UserEchoSettingsRepository interface {
	// GetByUserID returns the settings row, or nil if the user has no row yet.
	GetByUserID(ctx context.Context, userID uuid.UUID) (*entity.UserEchoSettings, error)
	// Create inserts a fresh row at revision 1 (the first write from the
	// implicit revision-0 default). A concurrent create for the same user
	// returns ErrEchoSettingsVersionConflict.
	Create(ctx context.Context, settings *entity.UserEchoSettings) error
	// Update applies a settings row guarded by the expected revision
	// (optimistic concurrency). It returns the new revision on success.
	Update(ctx context.Context, settings *entity.UserEchoSettings, expectedRevision int64) (int64, error)
}

// EchoRepository persists echo rows (one resurfacing = one memory card plus
// its reason). The service keeps a stable single-row "current echo" discipline
// over these primitives; the repository itself is a thin CRUD layer.
type EchoRepository interface {
	// Latest returns the user's most recent echo row (any status), or nil when
	// the user has no echo rows yet. It anchors the cadence gate: the next
	// echo is due cadence-days after this row's created_at.
	Latest(ctx context.Context, userID uuid.UUID) (*entity.Echo, error)
	// GetByID returns an echo belonging to the user, or ErrEchoNotFound when
	// the echo does not exist or belongs to another user.
	GetByID(ctx context.Context, userID uuid.UUID, echoID uuid.UUID) (*entity.Echo, error)
	// Create inserts a new echo row with status open.
	Create(ctx context.Context, echo *entity.Echo) error
	// UpdateStatus transitions a currently-open echo to the given status,
	// recording resolved_at. Only open rows can transition; a call on a row
	// that is not open returns ErrEchoNotOpen.
	UpdateStatus(ctx context.Context, userID uuid.UUID, echoID uuid.UUID, status entity.EchoStatus, resolvedAt *time.Time) error
	// FetchMemory returns the compact memory payload an echo points at, or
	// ErrEchoNotFound when the capture is not visible (deleted).
	FetchMemory(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.EchoMemory, error)
	// PickCandidate deterministically selects the next capture eligible to echo
	// (nil, nil when nothing is eligible). now anchors the cooldown windows:
	// not_relevant echoes exclude a capture for 90 days, any other resolved
	// status for 14 days. Selection orders pinned first, then least-recently
	// echoed, then oldest capture, then capture id as the tiebreaker.
	PickCandidate(ctx context.Context, userID uuid.UUID, now time.Time) (*entity.EchoCandidate, error)
}

// OutboxRepository manages the background processing lifecycle of
// CaptureOutbox events consumed by the enrichment worker.
type OutboxRepository interface {
	// ClaimDue atomically claims up to limit due events for processing,
	// returning them with status set to processing and attempt_count
	// incremented. Due events are queued/retry_wait rows whose next_run_at has
	// passed, plus stale processing rows whose updated_at lease has expired
	// (a crashed worker's orphans, re-claimed by another worker). lease is the
	// staleness window: processing rows not touched within the lease are
	// eligible for reclamation. FOR UPDATE SKIP LOCKED prevents two workers
	// claiming the same row.
	ClaimDue(ctx context.Context, limit int, lease time.Duration) ([]*entity.CaptureOutbox, error)
	// MarkReady marks a claimed event as successfully processed.
	MarkReady(ctx context.Context, id int64) error
	// MarkFailed records a terminal failure for a claimed event.
	MarkFailed(ctx context.Context, id int64, reason string) error
	// MarkRetryWait schedules a claimed event for a later run after backoff,
	// returning it to retry_wait with the given reason.
	MarkRetryWait(ctx context.Context, id int64, backoff time.Duration, reason string) error
	// CancelByUser transitions all queued/retry_wait events for a user to
	// cancelled. It returns the number of cancelled events. Used when the user
	// disables AI memory organizing.
	CancelByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	// CountQueued returns the number of queued/retry_wait events for a user.
	CountQueued(ctx context.Context, userID uuid.UUID) (int64, error)
	// CountPendingReorganize returns the number of events that were marked
	// ready without AI enrichment (policy_snapshot.ai_memory_enabled=false).
	// These are memories created while AI organizing was off and are eligible
	// for an explicit backfill once the user re-enables it.
	CountPendingReorganize(ctx context.Context, userID uuid.UUID) (int64, error)
	// ReorganizeByUser re-enqueues a user's ready events whose snapshot had AI
	// organizing disabled, stamping them with the current policy snapshot so
	// the worker enriches them. It returns the number of events re-enqueued.
	ReorganizeByUser(ctx context.Context, userID uuid.UUID, policyJSON []byte) (int64, error)
}

// ImportRepository persists batch import jobs and their rows. Rows reference
// captures via a nullable capture_id once committed. CreateJob inserts the job
// and its rows atomically in one transaction.
type ImportRepository interface {
	// CreateJob atomically inserts a draft job plus its rows.
	CreateJob(ctx context.Context, job *entity.ImportJob, rows []*entity.ImportRow) error
	// GetJob returns one job owned by the user, or ErrImportNotFound.
	GetJob(ctx context.Context, userID uuid.UUID, jobID uuid.UUID) (*entity.ImportJob, error)
	// ListRows returns one page of rows ordered by row_number.
	ListRows(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, limit, offset int) ([]*entity.ImportRow, error)
	// GetRowsByNumbers returns the given rows (by row_number) of a job.
	GetRowsByNumbers(ctx context.Context, userID uuid.UUID, jobID uuid.UUID, rowNumbers []int) ([]*entity.ImportRow, error)
	// UpdateJob persists a job's status, counts and commit/cancel timestamps.
	UpdateJob(ctx context.Context, job *entity.ImportJob) error
	// UpdateRow persists a row's mutable fields (status, dedupe, capture_id,
	// normalized payload, validation errors, content, content_hash).
	UpdateRow(ctx context.Context, row *entity.ImportRow) error
	// SetRowCompletion persists the AI field proposals for a row.
	SetRowCompletion(ctx context.Context, userID uuid.UUID, rowID uuid.UUID, proposals []entity.ImportFieldProposal) error
	// MarkRowState transitions a row to a terminal state, optionally recording
	// the created capture id and the imported timestamp.
	MarkRowState(ctx context.Context, userID uuid.UUID, rowID uuid.UUID, status entity.ImportRowStatus, captureID *uuid.UUID, importedAt *time.Time) error
	// FindExternalDuplicates returns the external_ids of non-deleted captures
	// matching any of the exact (source_name, external_id) dedup keys for the
	// user. Returning the keys (not capture ids) lets the caller mark exactly
	// which import rows are hard duplicates.
	FindExternalDuplicates(ctx context.Context, userID uuid.UUID, sourceName string, externalIDs []string) ([]string, error)
	// FindContentHashMatches returns the normalized content hashes of
	// non-deleted captures matching any of the given hashes for the user.
	FindContentHashMatches(ctx context.Context, userID uuid.UUID, hashes []string) ([]string, error)
}

// CompletionRepository persists AI field-completion proposals. One row is one
// FIELD suggestion for a Capture's MemoryCard; a preview batch shares PreviewID.
type CompletionRepository interface {
	// CreateProposals inserts a batch of field proposals for a capture,
	// stamping user_id/capture_id and returning the assigned ids and timestamps.
	CreateProposals(
		ctx context.Context,
		userID uuid.UUID,
		captureID uuid.UUID,
		proposals []*entity.CompletionProposal,
	) error
	// ListPendingByCapture returns a capture's proposals still in pending.
	ListPendingByCapture(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) ([]*entity.CompletionProposal, error)
	// ListByIDs returns the given proposals for a user, any status.
	ListByIDs(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) ([]*entity.CompletionProposal, error)
	// ExpireAllPending expires every pending proposal for a capture. Used when a
	// new preview supersedes the previous batch (only one active set survives).
	ExpireAllPending(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) error
	// MarkAccepted transitions pending proposals to accepted, recording who
	// accepted and when. Rows not in pending are left untouched (idempotent).
	MarkAccepted(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error
	// MarkRejected transitions pending proposals to rejected (a no-op when the
	// field became non-empty before apply, or the user declined).
	MarkRejected(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error
	// MarkExpired transitions pending proposals to expired (stale after a
	// source_revision change or a newer preview).
	MarkExpired(ctx context.Context, userID uuid.UUID, ids []uuid.UUID) error
}
