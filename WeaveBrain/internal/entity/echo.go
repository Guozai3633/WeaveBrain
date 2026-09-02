package entity

import (
	"time"

	"github.com/google/uuid"
)

// EchoCadence controls how often a new echo is offered. Cadence is enforced by
// the server at read time (anchored to the most recent echo row's created_at);
// the client only schedules local reminder notifications as an invitation.
type EchoCadence string

const (
	EchoCadenceDaily         EchoCadence = "daily"
	EchoCadenceEveryOtherDay EchoCadence = "every_other_day"
	EchoCadenceWeekly        EchoCadence = "weekly"
)

// EchoStatus is the lifecycle of a single echo row.
type EchoStatus string

const (
	EchoStatusOpen        EchoStatus = "open"         // current echo awaiting feedback
	EchoStatusDone        EchoStatus = "done"         // user answered 完成
	EchoStatusLater       EchoStatus = "later"        // user answered 稍后
	EchoStatusNotRelevant EchoStatus = "not_relevant" // user answered 无关
	EchoStatusExpired     EchoStatus = "expired"      // open past its cadence window
)

// EchoFeedbackVerdict is what the client reports for an open echo.
type EchoFeedbackVerdict string

const (
	EchoVerdictDone        EchoFeedbackVerdict = "done"
	EchoVerdictLater       EchoFeedbackVerdict = "later"
	EchoVerdictNotRelevant EchoFeedbackVerdict = "not_relevant"
)

// EchoReasonCode is the deterministic, explainable reason an echo was chosen.
// The human-facing reason text is re-derived from live values at read time.
type EchoReasonCode string

const (
	EchoReasonFirstEcho  EchoReasonCode = "first_echo"
	EchoReasonPinned     EchoReasonCode = "pinned"
	EchoReasonOldest     EchoReasonCode = "oldest"
	EchoReasonReminder   EchoReasonCode = "reminder"
)

// UserEchoSettings mirrors UserAISettings: server-owned switches guarded by an
// optimistic-concurrency revision. Echo is off by default (user opts in).
type UserEchoSettings struct {
	UserID    uuid.UUID   `json:"user_id"`
	Enabled   bool        `json:"enabled"`
	Cadence   EchoCadence `json:"cadence"`
	Revision  int64       `json:"revision"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// DefaultUserEchoSettings returns the defaults for a user with no row yet:
// disabled, daily cadence, revision 0.
func DefaultUserEchoSettings(userID uuid.UUID) *UserEchoSettings {
	return &UserEchoSettings{
		UserID:  userID,
		Enabled: false,
		Cadence: EchoCadenceDaily,
	}
}

// Echo is one resurfacing row: exactly one memory card plus the reason code
// that produced it.
type Echo struct {
	ID         uuid.UUID     `json:"id"`
	UserID     uuid.UUID     `json:"user_id"`
	CaptureID  uuid.UUID     `json:"capture_id"`
	Status     EchoStatus    `json:"status"`
	ReasonCode EchoReasonCode `json:"reason_code"`
	CreatedAt  time.Time     `json:"created_at"`
	UpdatedAt  time.Time     `json:"updated_at"`
	ResolvedAt *time.Time    `json:"resolved_at,omitempty"`
}

// EchoMemory is the compact memory payload shown on an echo card. CaptureID is
// the deep-link target (/memories/:captureId).
type EchoMemory struct {
	CaptureID   uuid.UUID  `json:"capture_id"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Summary     *string    `json:"summary,omitempty"`
	PrimaryType string     `json:"primary_type"`
	CapturedAt  *time.Time `json:"captured_at,omitempty"`
	IsPinned    bool       `json:"is_pinned"`
}

// EchoCandidate is the repository selection row for the next echo.
type EchoCandidate struct {
	Memory EchoMemory
	// HasPriorEcho is true when this capture was already echoed at least once
	// (any status). It steers reason classification.
	HasPriorEcho bool
}

// EchoReason pairs the stable code with its freshly rendered human text.
type EchoReason struct {
	Code EchoReasonCode `json:"code"`
	Text string         `json:"text"`
}

// EchoCadenceDays maps a cadence to its day step.
func (c EchoCadence) Days() int {
	switch c {
	case EchoCadenceEveryOtherDay:
		return 2
	case EchoCadenceWeekly:
		return 7
	default:
		return 1
	}
}
