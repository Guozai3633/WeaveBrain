package entity

import (
	"time"

	"github.com/google/uuid"
)

// EnrichmentSource records where a memory-card change originated. fallback is
// the local derivation at capture time, user is an explicit edit, and ai will
// be produced by the real AI pipeline in a later round.
type EnrichmentSource string

const (
	EnrichmentSourceFallback EnrichmentSource = "fallback"
	EnrichmentSourceAI       EnrichmentSource = "ai"
	EnrichmentSourceUser     EnrichmentSource = "user"
)

// EnrichmentRevision is an immutable revision of a MemoryCard's AI-claimed
// fields. source distinguishes local derivation (fallback) from user edits
// (user) and future AI organization (ai). source_revision points at the
// capture version the derivation was based on.
type EnrichmentRevision struct {
	ID            int64            `json:"id"`
	UserID        uuid.UUID        `json:"user_id"`
	CaptureID     uuid.UUID        `json:"capture_id"`
	Revision      int32            `json:"revision"`
	CardVersion   int64            `json:"card_version"`
	Source        EnrichmentSource `json:"source"`
	SourceRevision int64           `json:"source_revision"`
	Changes       map[string]any   `json:"changes"`
	Provenance    map[string]any   `json:"provenance"`
	CreatedAt     time.Time        `json:"created_at"`
}

// MemoryAggregate is the full detail view of a memory: the raw capture, its
// memory card, optional audio asset, latest transcript, and the card's
// revision trail. Audio/Transcript are nil when absent.
type MemoryAggregate struct {
	Capture    *Capture             `json:"capture"`
	MemoryCard *MemoryCard          `json:"memory_card"`
	Audio      *AudioAsset          `json:"audio,omitempty"`
	Transcript *TranscriptRevision  `json:"transcript,omitempty"`
	Revisions  []*EnrichmentRevision `json:"revisions"`
}

// MemoryListEntry is one row in the memory stream.
type MemoryListEntry struct {
	Capture    *Capture    `json:"capture"`
	MemoryCard *MemoryCard `json:"memory_card"`
}

// MemoryListQuery describes the memory stream filters and pagination.
// Cursor is an opaque, URL-safe keyset token returned by a previous page.
type MemoryListQuery struct {
	Cursor          *string
	Limit           int
	Q               string
	Kind            string
	PrimaryType     string
	LifecycleStatus string
	PinnedOnly      bool
}

// MemoryListResult is a page of memory stream entries.
type MemoryListResult struct {
	Items      []*MemoryListEntry
	NextCursor *string
}
