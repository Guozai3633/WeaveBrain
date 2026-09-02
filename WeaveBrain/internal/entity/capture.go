package entity

import (
	"time"

	"github.com/google/uuid"
)

type CaptureKind string

const (
	CaptureKindText   CaptureKind = "text"
	CaptureKindAudio  CaptureKind = "audio"
	CaptureKindImport CaptureKind = "import"
	CaptureKindShare  CaptureKind = "share"
)

type Capture struct {
	ID                  uuid.UUID   `json:"id"`
	UserID              uuid.UUID   `json:"user_id"`
	Kind                CaptureKind `json:"kind"`
	OriginalText        *string     `json:"original_text,omitempty"`
	CapturedAt          *time.Time  `json:"captured_at,omitempty"`
	CapturedAtPrecision string      `json:"captured_at_precision"`
	Timezone            *string     `json:"timezone,omitempty"`
	Source              string      `json:"source"`
	CollectionID        *int64      `json:"collection_id,omitempty"`
	PrivacyMode         string      `json:"privacy_mode"`
	ExternalID          *string     `json:"external_id,omitempty"`
	SourceName          *string     `json:"source_name,omitempty"`
	ContentHash         *string     `json:"-"`
	RequestHash         string      `json:"-"`
	ClientVersion       int         `json:"client_version"`
	Version             int64       `json:"version"`
	LifecycleStatus     string      `json:"lifecycle_status"`
	CreatedAt           time.Time   `json:"created_at"`
	UpdatedAt           time.Time   `json:"updated_at"`
}

type MemoryCard struct {
	ID               uuid.UUID `json:"id"`
	UserID           uuid.UUID `json:"user_id"`
	CaptureID        uuid.UUID `json:"capture_id"`
	PrimaryType      string    `json:"primary_type"`
	Title            string    `json:"title"`
	Summary          *string   `json:"summary,omitempty"`
	Tags             []string  `json:"tags,omitempty"`
	KeyPoints        []string  `json:"key_points,omitempty"`
	ProcessingStatus string    `json:"processing_status"`
	Version          int64     `json:"version"`
	IsPinned         bool      `json:"is_pinned"`
	PinnedAt         *time.Time `json:"pinned_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type CaptureOutbox struct {
	ID             int64           `json:"id"`
	UserID         uuid.UUID       `json:"user_id"`
	CaptureID      uuid.UUID       `json:"capture_id"`
	EventType      string          `json:"event_type"`
	Payload        map[string]any  `json:"payload"`
	PolicySnapshot *PolicySnapshot `json:"policy_snapshot,omitempty"`
	Status         string          `json:"status"`
	AttemptCount   int             `json:"attempt_count"`
	NextRunAt      time.Time       `json:"next_run_at"`
	CreatedAt      time.Time       `json:"created_at"`
	ProcessedAt    *time.Time      `json:"processed_at,omitempty"`
	LastError      *string         `json:"last_error,omitempty"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// PolicySnapshot captures the user's AI consent switches at the moment a
// Capture is created. The background worker applies the snapshot so later
// settings changes never retroactively trigger or suppress AI processing.
type PolicySnapshot struct {
	AIMemoryEnabled     bool `json:"ai_memory_enabled"`
	AICompletionEnabled bool `json:"ai_completion_enabled"`
	SpeechToTextEnabled bool `json:"speech_to_text_enabled"`
	CloudTextAllowed    bool `json:"cloud_text_allowed"`
	CloudAudioAllowed   bool `json:"cloud_audio_allowed"`
}

// DefaultPolicySnapshot is the privacy-first snapshot used when a user has no
// settings row: every AI feature stays off.
func DefaultPolicySnapshot() *PolicySnapshot {
	return &PolicySnapshot{}
}

// FromAISettings derives a snapshot from a user's current AI settings.
// A nil settings row falls back to the privacy-first defaults.
func FromAISettings(s *UserAISettings) *PolicySnapshot {
	if s == nil {
		return DefaultPolicySnapshot()
	}
	return &PolicySnapshot{
		AIMemoryEnabled:     s.AIMemoryEnabled,
		AICompletionEnabled: s.AICompletionEnabled,
		SpeechToTextEnabled: s.SpeechToTextEnabled,
		CloudTextAllowed:    s.CloudTextAllowed,
		CloudAudioAllowed:   s.CloudAudioAllowed,
	}
}

type CaptureAggregate struct {
	Capture    *Capture    `json:"capture"`
	MemoryCard *MemoryCard `json:"memory_card"`
}

// CaptureDedupe reports a content-hash duplicate hit during a single import.
// A non-nil ExistingCaptureID is a "suspected duplicate" — the import still
// proceeds; the client decides whether to keep or discard it.
type CaptureDedupe struct {
	Status            string     `json:"status"`
	ExistingCaptureID *uuid.UUID `json:"existing_capture_id,omitempty"`
}
