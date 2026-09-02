package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

var (
	// ErrInvalidEchoSettings reports an invalid cadence or a missing user id.
	ErrInvalidEchoSettings = errors.New("invalid echo settings")
	// ErrEchoSettingsConflict reports an optimistic-concurrency miss on the
	// echo settings row.
	ErrEchoSettingsConflict = errors.New("echo settings version conflict")
	// ErrInvalidEchoFeedback reports an unknown feedback verdict.
	ErrInvalidEchoFeedback = errors.New("invalid echo feedback")
)

// EchoCurrentCard is one offered echo: the selected memory plus the freshly
// rendered, human-readable reason.
type EchoCurrentCard struct {
	EchoID    uuid.UUID
	Status    entity.EchoStatus
	Reason    entity.EchoReason
	Memory    entity.EchoMemory
	CreatedAt time.Time
}

// EchoCurrentResult is the outcome of GET /echoes/current. Echo is nil when no
// echo is currently open (disabled / between cadence windows / no candidates).
type EchoCurrentResult struct {
	Enabled     bool
	Cadence     entity.EchoCadence
	Revision    int64
	Echo        *EchoCurrentCard
	NextDueAt   *time.Time
	EmptyReason string
}

// EchoFeedbackResult reports a successful feedback transition and when the
// next echo is due (cadence anchored to the resolved echo's created_at).
type EchoFeedbackResult struct {
	EchoID    uuid.UUID
	Status    entity.EchoStatus
	NextDueAt time.Time
}

// UpdateEchoSettingsInput is the validated patch for a user's echo settings.
// Pointer fields are applied only when present (partial update). Expected
// Revision is the client's last-seen revision for optimistic concurrency.
type UpdateEchoSettingsInput struct {
	UserID           uuid.UUID
	ExpectedRevision int64
	Enabled          *bool
	Cadence          *entity.EchoCadence
}

// EchoService owns the stable single-row "current echo" protocol. Every read of
// GET /echoes/current re-derives the answer from live state: the newest echo
// row anchors the cadence window; an open echo is reused while fresh, rolled to
// expired when stale; the next echo is created on demand when due. This keeps
// repeated opens and notification taps stable without a scheduler or push.
type EchoService struct {
	settings repository.UserEchoSettingsRepository
	echoes   repository.EchoRepository
	now      func() time.Time
}

// NewEchoService creates a new EchoService.
func NewEchoService(settings repository.UserEchoSettingsRepository, echoes repository.EchoRepository) *EchoService {
	return &EchoService{settings: settings, echoes: echoes, now: time.Now}
}

// GetSettings returns the effective echo settings, materializing the defaults
// (disabled, daily) when the user has no row yet.
func (s *EchoService) GetSettings(ctx context.Context, userID uuid.UUID) (*entity.UserEchoSettings, error) {
	if s == nil || s.settings == nil {
		return nil, fmt.Errorf("echo settings repository is not configured")
	}
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidEchoSettings)
	}
	stored, err := s.settings.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user echo settings: %w", err)
	}
	if stored == nil {
		return entity.DefaultUserEchoSettings(userID), nil
	}
	return stored, nil
}

// UpdateSettings applies a partial patch under optimistic concurrency. For a
// brand-new user (no row yet) the client must send ExpectedRevision 0; the row
// is then created with revision 1.
func (s *EchoService) UpdateSettings(
	ctx context.Context,
	input UpdateEchoSettingsInput,
) (*entity.UserEchoSettings, error) {
	if s == nil || s.settings == nil {
		return nil, fmt.Errorf("echo settings repository is not configured")
	}
	if input.UserID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidEchoSettings)
	}
	if input.Cadence != nil {
		switch *input.Cadence {
		case entity.EchoCadenceDaily, entity.EchoCadenceEveryOtherDay, entity.EchoCadenceWeekly:
		default:
			return nil, fmt.Errorf("%w: unsupported cadence %q", ErrInvalidEchoSettings, *input.Cadence)
		}
	}

	existing, err := s.GetSettings(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	if input.ExpectedRevision != existing.Revision {
		return nil, fmt.Errorf(
			"%w: expected=%d current=%d",
			ErrEchoSettingsConflict,
			input.ExpectedRevision,
			existing.Revision,
		)
	}

	if existing.CreatedAt.IsZero() {
		// No persisted row yet: materialize the defaults and insert.
		existing = entity.DefaultUserEchoSettings(input.UserID)
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	if input.Cadence != nil {
		existing.Cadence = *input.Cadence
	}

	if existing.CreatedAt.IsZero() {
		if err := s.settings.Create(ctx, existing); err != nil {
			if errors.Is(err, repository.ErrEchoSettingsVersionConflict) {
				return nil, ErrEchoSettingsConflict
			}
			return nil, fmt.Errorf("create user echo settings: %w", err)
		}
		updated, err := s.GetSettings(ctx, input.UserID)
		if err != nil {
			return nil, err
		}
		return updated, nil
	}

	newRevision, err := s.settings.Update(ctx, existing, input.ExpectedRevision)
	if err != nil {
		if errors.Is(err, repository.ErrEchoSettingsVersionConflict) {
			return nil, ErrEchoSettingsConflict
		}
		return nil, fmt.Errorf("update user echo settings: %w", err)
	}
	existing.Revision = newRevision
	return existing, nil
}

// CurrentEcho returns the current echo for a user, creating one on demand when
// the cadence window has opened. It never mutates settings and never fails for
// a healthy user: disabled, between windows, and no-candidate states all come
// back as explicit empty results.
func (s *EchoService) CurrentEcho(ctx context.Context, userID uuid.UUID) (*EchoCurrentResult, error) {
	if s == nil || s.settings == nil || s.echoes == nil {
		return nil, fmt.Errorf("echo service is not configured")
	}
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidEchoSettings)
	}
	now := s.now()

	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return &EchoCurrentResult{
			Enabled:  false,
			Cadence:  settings.Cadence,
			Revision: settings.Revision,
		}, nil
	}

	latest, err := s.echoes.Latest(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get latest echo: %w", err)
	}

	// retiredInCall is true when an open echo was rolled to expired within this
	// request. Its created_at window is not a natural cadence anchor yet, so the
	// cadence gate below is bypassed and a replacement is offered immediately.
	retiredInCall := false
	if latest != nil && latest.Status == entity.EchoStatusOpen {
		dueAt := latest.CreatedAt.AddDate(0, 0, settings.Cadence.Days())
		if now.Before(dueAt) {
			// Stable window: reuse the existing open echo so repeated opens and
			// notification taps return the same card.
			result, err := s.buildCurrent(ctx, userID, latest, settings)
			if err == nil {
				return result, nil
			}
			if !errors.Is(err, repository.ErrEchoNotFound) {
				return nil, err
			}
			// The capture behind the open echo is no longer visible; expire the
			// orphaned echo and offer a replacement immediately.
			if err := s.expire(ctx, userID, latest.ID, now); err != nil {
				return nil, err
			}
			latest.Status = entity.EchoStatusExpired
			retiredInCall = true
		} else {
			// Stale open past its cadence window: roll it to expired.
			if err := s.expire(ctx, userID, latest.ID, now); err != nil {
				return nil, err
			}
			latest.Status = entity.EchoStatusExpired
			retiredInCall = true
		}
	}

	// Cadence gate anchored to the most recent pre-existing echo row's
	// created_at: a new echo is offered only once cadence days have passed since
	// the previous one (of any status) was created. A row retired within this
	// call is skipped so an expired/unresolvable open echo yields its successor
	// rather than an empty wait state.
	if !retiredInCall && latest != nil {
		nextDueAt := latest.CreatedAt.AddDate(0, 0, settings.Cadence.Days())
		if now.Before(nextDueAt) {
			return &EchoCurrentResult{
				Enabled:   true,
				Cadence:   settings.Cadence,
				Revision:  settings.Revision,
				NextDueAt: &nextDueAt,
			}, nil
		}
	}

	candidate, err := s.echoes.PickCandidate(ctx, userID, now)
	if err != nil {
		return nil, fmt.Errorf("pick echo candidate: %w", err)
	}
	if candidate == nil {
		return &EchoCurrentResult{
			Enabled:     true,
			Cadence:     settings.Cadence,
			Revision:    settings.Revision,
			EmptyReason: "no_candidates",
		}, nil
	}

	echo := &entity.Echo{
		ID:         uuid.New(),
		UserID:     userID,
		CaptureID:  candidate.Memory.CaptureID,
		Status:     entity.EchoStatusOpen,
		ReasonCode: classifyEchoReason(latest, candidate),
	}
	if err := s.echoes.Create(ctx, echo); err != nil {
		return nil, fmt.Errorf("create echo: %w", err)
	}
	return s.buildCurrent(ctx, userID, echo, settings)
}

// Feedback records the user's verdict for the given open echo. done/later/
// not_relevant each resolve the echo; an echo that is not open (already
// resolved, or expired) is rejected so multi-device races surface as conflicts.
func (s *EchoService) Feedback(
	ctx context.Context,
	userID uuid.UUID,
	echoID uuid.UUID,
	verdict entity.EchoFeedbackVerdict,
) (*EchoFeedbackResult, error) {
	if s == nil || s.settings == nil || s.echoes == nil {
		return nil, fmt.Errorf("echo service is not configured")
	}
	if userID == uuid.Nil || echoID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id and echo_id are required", ErrInvalidEchoSettings)
	}
	status, ok := echoStatusForVerdict(verdict)
	if !ok {
		return nil, fmt.Errorf("%w: unsupported verdict %q", ErrInvalidEchoFeedback, verdict)
	}

	echo, err := s.echoes.GetByID(ctx, userID, echoID)
	if err != nil {
		return nil, err
	}
	if echo.Status != entity.EchoStatusOpen {
		return nil, repository.ErrEchoNotOpen
	}

	now := s.now()
	if err := s.echoes.UpdateStatus(ctx, userID, echoID, status, &now); err != nil {
		return nil, err
	}

	settings, err := s.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &EchoFeedbackResult{
		EchoID:    echoID,
		Status:    status,
		NextDueAt: echo.CreatedAt.AddDate(0, 0, settings.Cadence.Days()),
	}, nil
}

// buildCurrent assembles the current-echo payload for an open echo row.
func (s *EchoService) buildCurrent(
	ctx context.Context,
	userID uuid.UUID,
	echo *entity.Echo,
	settings *entity.UserEchoSettings,
) (*EchoCurrentResult, error) {
	memory, err := s.echoes.FetchMemory(ctx, userID, echo.CaptureID)
	if err != nil {
		return nil, err
	}
	return &EchoCurrentResult{
		Enabled:  true,
		Cadence:  settings.Cadence,
		Revision: settings.Revision,
		Echo: &EchoCurrentCard{
			EchoID:    echo.ID,
			Status:    echo.Status,
			Reason:    entity.EchoReason{Code: echo.ReasonCode, Text: echoReasonText(echo.ReasonCode)},
			Memory:    *memory,
			CreatedAt: echo.CreatedAt,
		},
	}, nil
}

// expire rolls an open echo to expired, recording resolved_at at now.
func (s *EchoService) expire(ctx context.Context, userID uuid.UUID, echoID uuid.UUID, at time.Time) error {
	if err := s.echoes.UpdateStatus(ctx, userID, echoID, entity.EchoStatusExpired, &at); err != nil {
		if errors.Is(err, repository.ErrEchoNotOpen) {
			// A concurrent feedback already resolved it; treat as expired.
			return nil
		}
		return fmt.Errorf("expire echo: %w", err)
	}
	return nil
}

// classifyEchoReason maps a created echo to its deterministic reason code. The
// user's very first echo is first_echo; afterwards the selected card drives the
// classification (pinned, never-echoed oldest, or a repeat reminder).
func classifyEchoReason(latest *entity.Echo, candidate *entity.EchoCandidate) entity.EchoReasonCode {
	if latest == nil {
		return entity.EchoReasonFirstEcho
	}
	if candidate.Memory.IsPinned {
		return entity.EchoReasonPinned
	}
	if !candidate.HasPriorEcho {
		return entity.EchoReasonOldest
	}
	return entity.EchoReasonReminder
}

// echoStatusForVerdict maps a client feedback verdict to its echo status.
func echoStatusForVerdict(v entity.EchoFeedbackVerdict) (entity.EchoStatus, bool) {
	switch v {
	case entity.EchoVerdictDone:
		return entity.EchoStatusDone, true
	case entity.EchoVerdictLater:
		return entity.EchoStatusLater, true
	case entity.EchoVerdictNotRelevant:
		return entity.EchoStatusNotRelevant, true
	default:
		return "", false
	}
}

// echoReasonText renders the human-readable reason for a reason code. The text
// is re-derived live from the code so it always stays fresh.
func echoReasonText(code entity.EchoReasonCode) string {
	switch code {
	case entity.EchoReasonFirstEcho:
		return "从你较早记下、还没回看过的记忆开始"
	case entity.EchoReasonPinned:
		return "这条记忆被你置顶过，适合专门回看"
	case entity.EchoReasonOldest:
		return "这是你较早记下、还没有回看过的想法"
	case entity.EchoReasonReminder:
		return "距离上次看到它已经过了一段时间，再想想也许有新角度"
	default:
		return ""
	}
}
