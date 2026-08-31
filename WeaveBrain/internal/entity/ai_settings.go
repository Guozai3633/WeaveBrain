package entity

import (
	"time"

	"github.com/google/uuid"
)

// UserAISettings captures the AI-optional consent switches for a user.
//
// Every switch defaults to false (privacy-first / AI-optional). revision is
// incremented on every update so multiple devices can detect stale writes with
// optimistic concurrency (VERSION_CONFLICT).
type UserAISettings struct {
	UserID              uuid.UUID `json:"user_id"`
	AIMemoryEnabled     bool      `json:"ai_memory_enabled"`
	AICompletionEnabled bool      `json:"ai_completion_enabled"`
	SpeechToTextEnabled bool      `json:"speech_to_text_enabled"`
	CloudTextAllowed    bool      `json:"cloud_text_allowed"`
	CloudAudioAllowed   bool      `json:"cloud_audio_allowed"`
	Revision            int64     `json:"revision"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// DefaultUserAISettings returns the privacy-first defaults for a user (all
// switches false, revision 0). It is used when a user has no persisted row yet.
func DefaultUserAISettings(userID uuid.UUID) *UserAISettings {
	return &UserAISettings{
		UserID: userID,
	}
}
