package entity

import (
	"time"

	"github.com/google/uuid"
)

type AudioUploadState string

const (
	AudioUploadStateInitiated AudioUploadState = "initiated"
	AudioUploadStateUploading AudioUploadState = "uploading"
	AudioUploadStateComplete  AudioUploadState = "complete"
	AudioUploadStateFailed    AudioUploadState = "failed"
)

// AudioAsset is a single audio file attached to a Capture. The upload is
// chunked and checksum-verified; upload_state tracks the upload lifecycle.
type AudioAsset struct {
	ID             uuid.UUID         `json:"id"`
	UserID         uuid.UUID         `json:"user_id"`
	CaptureID      uuid.UUID         `json:"capture_id"`
	MimeType       string            `json:"mime_type"`
	DurationMs     *int64            `json:"duration_ms,omitempty"`
	SizeBytes      int64             `json:"size_bytes"`
	SHA256         *string           `json:"sha256,omitempty"`
	StoragePath    string            `json:"-"`
	UploadState    AudioUploadState  `json:"upload_state"`
	TotalChunks    int32             `json:"total_chunks"`
	ReceivedChunks int32             `json:"received_chunks"`
	STTEnabled     bool              `json:"stt_enabled"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type TranscriptSource string

const (
	TranscriptSourceSTT  TranscriptSource = "stt"
	TranscriptSourceUser TranscriptSource = "user"
)

// TranscriptRevision is an immutable revision of a Capture's transcript.
// source distinguishes automatic (stt) from user-corrected (user) text.
type TranscriptRevision struct {
	ID        int64            `json:"id"`
	UserID    uuid.UUID        `json:"user_id"`
	CaptureID uuid.UUID        `json:"capture_id"`
	Revision  int32            `json:"revision"`
	Text      string           `json:"text"`
	Source    TranscriptSource `json:"source"`
	Confidence *float32        `json:"confidence,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

// CaptureAudioAggregate extends the capture view with its audio asset and the
// latest transcript revision.
type CaptureAudioAggregate struct {
	Capture    *Capture              `json:"capture"`
	MemoryCard *MemoryCard           `json:"memory_card"`
	Audio      *AudioAsset           `json:"audio,omitempty"`
	Transcript *TranscriptRevision   `json:"transcript,omitempty"`
}
