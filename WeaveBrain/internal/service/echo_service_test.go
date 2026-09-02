package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type fakeEchoSettingsRepo struct {
	stored         *entity.UserEchoSettings
	createConflict bool
	updateConflict bool
}

func (f *fakeEchoSettingsRepo) GetByUserID(
	_ context.Context,
	userID uuid.UUID,
) (*entity.UserEchoSettings, error) {
	if f.stored == nil {
		return nil, nil
	}
	cp := *f.stored
	return &cp, nil
}

func (f *fakeEchoSettingsRepo) Create(_ context.Context, s *entity.UserEchoSettings) error {
	if f.createConflict {
		return repository.ErrEchoSettingsVersionConflict
	}
	cp := *s
	cp.Revision = 1
	now := time.Now().UTC()
	cp.CreatedAt = now
	cp.UpdatedAt = now
	f.stored = &cp
	return nil
}

func (f *fakeEchoSettingsRepo) Update(
	_ context.Context,
	s *entity.UserEchoSettings,
	expectedRevision int64,
) (int64, error) {
	if f.updateConflict {
		return 0, repository.ErrEchoSettingsVersionConflict
	}
	cp := *s
	cp.Revision = expectedRevision + 1
	cp.UpdatedAt = time.Now().UTC()
	f.stored = &cp
	return cp.Revision, nil
}

type echoUpdateCall struct {
	echoID     uuid.UUID
	status     entity.EchoStatus
	resolvedAt *time.Time
}

type fakeEchoRepo struct {
	latestFn        func(ctx context.Context, userID uuid.UUID) (*entity.Echo, error)
	getByIDFn       func(ctx context.Context, userID uuid.UUID, echoID uuid.UUID) (*entity.Echo, error)
	createFn        func(ctx context.Context, echo *entity.Echo) error
	updateStatusFn  func(ctx context.Context, userID uuid.UUID, echoID uuid.UUID, status entity.EchoStatus, resolvedAt *time.Time) error
	fetchMemoryFn   func(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.EchoMemory, error)
	pickFn          func(ctx context.Context, userID uuid.UUID, now time.Time) (*entity.EchoCandidate, error)

	created []*entity.Echo
	updates []echoUpdateCall
}

func (f *fakeEchoRepo) Latest(ctx context.Context, userID uuid.UUID) (*entity.Echo, error) {
	if f.latestFn == nil {
		panic("unexpected EchoRepository.Latest")
	}
	return f.latestFn(ctx, userID)
}

func (f *fakeEchoRepo) GetByID(ctx context.Context, userID uuid.UUID, echoID uuid.UUID) (*entity.Echo, error) {
	if f.getByIDFn == nil {
		panic("unexpected EchoRepository.GetByID")
	}
	return f.getByIDFn(ctx, userID, echoID)
}

func (f *fakeEchoRepo) Create(ctx context.Context, echo *entity.Echo) error {
	f.created = append(f.created, echo)
	if f.createFn != nil {
		return f.createFn(ctx, echo)
	}
	return nil
}

func (f *fakeEchoRepo) UpdateStatus(ctx context.Context, userID uuid.UUID, echoID uuid.UUID, status entity.EchoStatus, resolvedAt *time.Time) error {
	f.updates = append(f.updates, echoUpdateCall{echoID: echoID, status: status, resolvedAt: resolvedAt})
	if f.updateStatusFn != nil {
		return f.updateStatusFn(ctx, userID, echoID, status, resolvedAt)
	}
	return nil
}

func (f *fakeEchoRepo) FetchMemory(ctx context.Context, userID uuid.UUID, captureID uuid.UUID) (*entity.EchoMemory, error) {
	if f.fetchMemoryFn == nil {
		panic("unexpected EchoRepository.FetchMemory")
	}
	return f.fetchMemoryFn(ctx, userID, captureID)
}

func (f *fakeEchoRepo) PickCandidate(ctx context.Context, userID uuid.UUID, now time.Time) (*entity.EchoCandidate, error) {
	if f.pickFn == nil {
		panic("unexpected EchoRepository.PickCandidate")
	}
	return f.pickFn(ctx, userID, now)
}

func fixedClock() time.Time {
	return time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
}

func newEchoSvc(settings *fakeEchoSettingsRepo, echoes *fakeEchoRepo) *EchoService {
	svc := NewEchoService(settings, echoes)
	svc.now = fixedClock
	return svc
}

func echoSettingsRow(userID uuid.UUID, enabled bool, cadence entity.EchoCadence, revision int64) *entity.UserEchoSettings {
	now := time.Now().UTC()
	return &entity.UserEchoSettings{
		UserID: userID, Enabled: enabled, Cadence: cadence, Revision: revision,
		CreatedAt: now, UpdatedAt: now,
	}
}

func sampleEchoMemory(captureID uuid.UUID) *entity.EchoMemory {
	return &entity.EchoMemory{
		CaptureID: captureID, Kind: "text", Title: "织脑的想法",
		PrimaryType: "idea",
	}
}

// --- CurrentEcho ---

func TestEchoServiceCurrentDisabledReturnsEmpty(t *testing.T) {
	userID := uuid.New()
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, &fakeEchoRepo{})

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if res.Enabled {
		t.Fatalf("default must be disabled, got %#v", res)
	}
	if res.Echo != nil {
		t.Fatalf("disabled must not offer an echo, got %#v", res.Echo)
	}
}

func TestEchoServiceCurrentStableOpenEchoIsReused(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	now := fixedClock()
	openEcho := &entity.Echo{
		ID: uuid.New(), UserID: userID, CaptureID: captureID,
		Status: entity.EchoStatusOpen, ReasonCode: entity.EchoReasonReminder,
		CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour),
	}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return openEcho, nil },
		fetchMemoryFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.EchoMemory, error) {
			return sampleEchoMemory(captureID), nil
		},
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if res.Echo == nil || res.Echo.EchoID != openEcho.ID {
		t.Fatalf("expected the same open echo, got %#v", res.Echo)
	}
	if len(echoes.created) != 0 || len(echoes.updates) != 0 {
		t.Fatalf("stable open echo must not create or transition rows")
	}
	if res.Echo.Reason.Code != entity.EchoReasonReminder || res.Echo.Reason.Text == "" {
		t.Fatalf("echo must carry a rendered reason, got %#v", res.Echo.Reason)
	}
}

func TestEchoServiceCurrentStaleOpenExpiresThenCreatesNext(t *testing.T) {
	userID := uuid.New()
	now := fixedClock()
	staleID := uuid.New()
	captureID := uuid.New()
	staleOpen := &entity.Echo{
		ID: staleID, UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusOpen, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: now.AddDate(0, 0, -2), UpdatedAt: now.AddDate(0, 0, -2),
	}
	cand := &entity.EchoCandidate{Memory: *sampleEchoMemory(captureID), HasPriorEcho: false}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return staleOpen, nil },
		fetchMemoryFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.EchoMemory, error) {
			return sampleEchoMemory(captureID), nil
		},
		pickFn: func(context.Context, uuid.UUID, time.Time) (*entity.EchoCandidate, error) { return cand, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if len(echoes.updates) != 1 || echoes.updates[0].echoID != staleID ||
		echoes.updates[0].status != entity.EchoStatusExpired {
		t.Fatalf("expected stale open to be expired, got %#v", echoes.updates)
	}
	if len(echoes.created) != 1 || echoes.created[0].CaptureID != captureID {
		t.Fatalf("expected one new open echo for the candidate, got %#v", echoes.created)
	}
	if res.Echo == nil || res.Echo.Memory.CaptureID != captureID {
		t.Fatalf("expected the freshly created echo to be current, got %#v", res.Echo)
	}
	if res.Echo.Reason.Code != entity.EchoReasonOldest {
		t.Fatalf("expected oldest reason for a never-echoed card, got %#v", res.Echo.Reason)
	}
}

func TestEchoServiceCurrentBetweenWindowsReturnsNextDue(t *testing.T) {
	userID := uuid.New()
	now := fixedClock()
	doneEcho := &entity.Echo{
		ID: uuid.New(), UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusDone, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: now.Add(-10 * time.Hour), UpdatedAt: now.Add(-10 * time.Hour),
	}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return doneEcho, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if res.Echo != nil {
		t.Fatalf("between windows must not offer an echo, got %#v", res.Echo)
	}
	wantNext := doneEcho.CreatedAt.AddDate(0, 0, 1)
	if res.NextDueAt == nil || !res.NextDueAt.Equal(wantNext) {
		t.Fatalf("expected next_due_at %v, got %v", wantNext, res.NextDueAt)
	}
	if len(echoes.created) != 0 {
		t.Fatalf("between windows must not create an echo")
	}
}

func TestEchoServiceCurrentNoCandidateReturnsEmptyReason(t *testing.T) {
	userID := uuid.New()
	now := fixedClock()
	doneEcho := &entity.Echo{
		ID: uuid.New(), UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusDone, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: now.AddDate(0, 0, -3), UpdatedAt: now.AddDate(0, 0, -3),
	}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return doneEcho, nil },
		pickFn: func(context.Context, uuid.UUID, time.Time) (*entity.EchoCandidate, error) {
			return nil, nil
		},
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if res.Echo != nil || res.EmptyReason != "no_candidates" {
		t.Fatalf("expected no_candidates empty reason, got %#v", res)
	}
}

func TestEchoServiceCurrentFirstEchoUsesFirstEchoReason(t *testing.T) {
	userID := uuid.New()
	captureID := uuid.New()
	cand := &entity.EchoCandidate{Memory: *sampleEchoMemory(captureID), HasPriorEcho: false}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return nil, nil },
		fetchMemoryFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.EchoMemory, error) {
			return sampleEchoMemory(captureID), nil
		},
		pickFn: func(context.Context, uuid.UUID, time.Time) (*entity.EchoCandidate, error) { return cand, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if len(echoes.created) != 1 || echoes.created[0].ReasonCode != entity.EchoReasonFirstEcho {
		t.Fatalf("first-ever echo must be first_echo, got %#v", echoes.created)
	}
	if res.Echo == nil || res.Echo.Reason.Code != entity.EchoReasonFirstEcho {
		t.Fatalf("expected first_echo reason rendered, got %#v", res.Echo)
	}
}

func TestEchoServiceCurrentPinnedCandidateReason(t *testing.T) {
	userID := uuid.New()
	now := fixedClock()
	captureID := uuid.New()
	doneEcho := &entity.Echo{
		ID: uuid.New(), UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusDone, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: now.AddDate(0, 0, -3), UpdatedAt: now.AddDate(0, 0, -3),
	}
	mem := sampleEchoMemory(captureID)
	mem.IsPinned = true
	cand := &entity.EchoCandidate{Memory: *mem, HasPriorEcho: false}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return doneEcho, nil },
		fetchMemoryFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.EchoMemory, error) {
			return mem, nil
		},
		pickFn: func(context.Context, uuid.UUID, time.Time) (*entity.EchoCandidate, error) { return cand, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if len(echoes.created) != 1 || echoes.created[0].ReasonCode != entity.EchoReasonPinned {
		t.Fatalf("pinned candidate must be pinned reason, got %#v", echoes.created)
	}
	if res.Echo == nil || res.Echo.Reason.Code != entity.EchoReasonPinned {
		t.Fatalf("expected pinned reason rendered, got %#v", res.Echo)
	}
}

func TestEchoServiceCurrentRepeatCandidateReason(t *testing.T) {
	userID := uuid.New()
	now := fixedClock()
	captureID := uuid.New()
	doneEcho := &entity.Echo{
		ID: uuid.New(), UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusDone, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30),
	}
	cand := &entity.EchoCandidate{Memory: *sampleEchoMemory(captureID), HasPriorEcho: true}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return doneEcho, nil },
		fetchMemoryFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.EchoMemory, error) {
			return sampleEchoMemory(captureID), nil
		},
		pickFn: func(context.Context, uuid.UUID, time.Time) (*entity.EchoCandidate, error) { return cand, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if len(echoes.created) != 1 || echoes.created[0].ReasonCode != entity.EchoReasonReminder {
		t.Fatalf("repeated card must be reminder reason, got %#v", echoes.created)
	}
	if res.Echo == nil || res.Echo.Reason.Code != entity.EchoReasonReminder {
		t.Fatalf("expected reminder reason rendered, got %#v", res.Echo)
	}
}

func TestEchoServiceCurrentOrphanOpenEchoSelfHeals(t *testing.T) {
	userID := uuid.New()
	now := fixedClock()
	staleID := uuid.New()
	captureID := uuid.New()
	orphanCaptureID := uuid.New()
	openEcho := &entity.Echo{
		ID: staleID, UserID: userID, CaptureID: orphanCaptureID,
		Status: entity.EchoStatusOpen, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour),
	}
	cand := &entity.EchoCandidate{Memory: *sampleEchoMemory(captureID), HasPriorEcho: false}
	echoes := &fakeEchoRepo{
		latestFn: func(context.Context, uuid.UUID) (*entity.Echo, error) { return openEcho, nil },
		fetchMemoryFn: func(_ context.Context, _ uuid.UUID, captureID uuid.UUID) (*entity.EchoMemory, error) {
			if captureID == orphanCaptureID {
				return nil, repository.ErrEchoNotFound
			}
			return sampleEchoMemory(captureID), nil
		},
		pickFn: func(context.Context, uuid.UUID, time.Time) (*entity.EchoCandidate, error) { return cand, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.CurrentEcho(context.Background(), userID)
	if err != nil {
		t.Fatalf("CurrentEcho: %v", err)
	}
	if len(echoes.updates) != 1 || echoes.updates[0].echoID != staleID ||
		echoes.updates[0].status != entity.EchoStatusExpired {
		t.Fatalf("expected orphaned open echo to be expired, got %#v", echoes.updates)
	}
	if res.Echo == nil || res.Echo.Memory.CaptureID != captureID {
		t.Fatalf("expected a replacement echo to be offered, got %#v", res.Echo)
	}
}

// --- Feedback ---

func TestEchoServiceFeedbackDoneTransitionsAndComputesNextDue(t *testing.T) {
	userID := uuid.New()
	now := fixedClock()
	echoID := uuid.New()
	openEcho := &entity.Echo{
		ID: echoID, UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusOpen, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: now.Add(-10 * time.Hour), UpdatedAt: now.Add(-10 * time.Hour),
	}
	echoes := &fakeEchoRepo{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.Echo, error) { return openEcho, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}, echoes)

	res, err := svc.Feedback(context.Background(), userID, echoID, entity.EchoVerdictDone)
	if err != nil {
		t.Fatalf("Feedback: %v", err)
	}
	if res.Status != entity.EchoStatusDone || res.EchoID != echoID {
		t.Fatalf("unexpected feedback result: %#v", res)
	}
	if len(echoes.updates) != 1 || echoes.updates[0].echoID != echoID ||
		echoes.updates[0].status != entity.EchoStatusDone || echoes.updates[0].resolvedAt == nil {
		t.Fatalf("expected done transition with resolved_at, got %#v", echoes.updates)
	}
	wantNext := openEcho.CreatedAt.AddDate(0, 0, 1)
	if !res.NextDueAt.Equal(wantNext) {
		t.Fatalf("expected next_due_at %v, got %v", wantNext, res.NextDueAt)
	}
}

func TestEchoServiceFeedbackNotOpenReturnsConflict(t *testing.T) {
	userID := uuid.New()
	doneEcho := &entity.Echo{
		ID: uuid.New(), UserID: userID, CaptureID: uuid.New(),
		Status: entity.EchoStatusDone, ReasonCode: entity.EchoReasonFirstEcho,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	echoes := &fakeEchoRepo{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.Echo, error) { return doneEcho, nil },
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, echoes)

	_, err := svc.Feedback(context.Background(), userID, doneEcho.ID, entity.EchoVerdictLater)
	if !errors.Is(err, repository.ErrEchoNotOpen) {
		t.Fatalf("expected ErrEchoNotOpen, got %v", err)
	}
	if len(echoes.updates) != 0 {
		t.Fatalf("non-open echo must not be updated")
	}
}

func TestEchoServiceFeedbackUnknownEchoPropagatesNotFound(t *testing.T) {
	userID := uuid.New()
	echoes := &fakeEchoRepo{
		getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*entity.Echo, error) {
			return nil, repository.ErrEchoNotFound
		},
	}
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, echoes)

	_, err := svc.Feedback(context.Background(), userID, uuid.New(), entity.EchoVerdictDone)
	if !errors.Is(err, repository.ErrEchoNotFound) {
		t.Fatalf("expected ErrEchoNotFound, got %v", err)
	}
}

func TestEchoServiceFeedbackInvalidVerdictRejected(t *testing.T) {
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, &fakeEchoRepo{})
	_, err := svc.Feedback(context.Background(), uuid.New(), uuid.New(), "snooze")
	if !errors.Is(err, ErrInvalidEchoFeedback) {
		t.Fatalf("expected ErrInvalidEchoFeedback, got %v", err)
	}
}

func TestEchoServiceFeedbackRequiresUserID(t *testing.T) {
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, &fakeEchoRepo{})
	_, err := svc.Feedback(context.Background(), uuid.Nil, uuid.New(), entity.EchoVerdictDone)
	if !errors.Is(err, ErrInvalidEchoSettings) {
		t.Fatalf("expected ErrInvalidEchoSettings, got %v", err)
	}
}

// --- Settings ---

func TestEchoSettingsGetMaterializesDefaultsWhenNoRow(t *testing.T) {
	userID := uuid.New()
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, &fakeEchoRepo{})

	got, err := svc.GetSettings(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if got.UserID != userID {
		t.Fatalf("unexpected user_id %v", got.UserID)
	}
	if got.Enabled {
		t.Fatalf("default must be disabled, got %#v", got)
	}
	if got.Cadence != entity.EchoCadenceDaily {
		t.Fatalf("default cadence must be daily, got %#v", got)
	}
	if got.Revision != 0 {
		t.Fatalf("expected default revision 0, got %d", got.Revision)
	}
}

func TestEchoSettingsGetReturnsStoredRow(t *testing.T) {
	userID := uuid.New()
	svc := newEchoSvc(&fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceWeekly, 7)}, &fakeEchoRepo{})

	got, err := svc.GetSettings(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if !got.Enabled || got.Cadence != entity.EchoCadenceWeekly || got.Revision != 7 {
		t.Fatalf("expected stored values, got %#v", got)
	}
}

func TestEchoSettingsUpdateNewUserCreatesRowAtRevisionOne(t *testing.T) {
	userID := uuid.New()
	repo := &fakeEchoSettingsRepo{}
	svc := newEchoSvc(repo, &fakeEchoRepo{})
	enabled := true

	got, err := svc.UpdateSettings(context.Background(), UpdateEchoSettingsInput{
		UserID: userID, ExpectedRevision: 0, Enabled: &enabled,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if !got.Enabled || got.Revision != 1 {
		t.Fatalf("expected enabled revision 1, got %#v", got)
	}
	if repo.stored == nil {
		t.Fatal("expected row to be created")
	}
}

func TestEchoSettingsUpdateMatchesRevisionBumpsRevision(t *testing.T) {
	userID := uuid.New()
	repo := &fakeEchoSettingsRepo{stored: echoSettingsRow(userID, false, entity.EchoCadenceDaily, 2)}
	svc := newEchoSvc(repo, &fakeEchoRepo{})
	cadence := entity.EchoCadenceEveryOtherDay

	got, err := svc.UpdateSettings(context.Background(), UpdateEchoSettingsInput{
		UserID: userID, ExpectedRevision: 2, Cadence: &cadence,
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if got.Revision != 3 {
		t.Fatalf("expected revision 3, got %d", got.Revision)
	}
	if got.Cadence != entity.EchoCadenceEveryOtherDay {
		t.Fatalf("expected cadence patched, got %#v", got)
	}
}

func TestEchoSettingsUpdateStaleRevisionReturnsVersionConflict(t *testing.T) {
	userID := uuid.New()
	repo := &fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 5)}
	svc := newEchoSvc(repo, &fakeEchoRepo{})
	enabled := true

	_, err := svc.UpdateSettings(context.Background(), UpdateEchoSettingsInput{
		UserID: userID, ExpectedRevision: 3, Enabled: &enabled,
	})
	if !errors.Is(err, ErrEchoSettingsConflict) {
		t.Fatalf("expected ErrEchoSettingsConflict, got %v", err)
	}
}

func TestEchoSettingsUpdateInvalidCadenceRejected(t *testing.T) {
	userID := uuid.New()
	repo := &fakeEchoSettingsRepo{stored: echoSettingsRow(userID, true, entity.EchoCadenceDaily, 1)}
	svc := newEchoSvc(repo, &fakeEchoRepo{})
	cadence := entity.EchoCadence("monthly")

	_, err := svc.UpdateSettings(context.Background(), UpdateEchoSettingsInput{
		UserID: userID, ExpectedRevision: 1, Cadence: &cadence,
	})
	if !errors.Is(err, ErrInvalidEchoSettings) {
		t.Fatalf("expected ErrInvalidEchoSettings, got %v", err)
	}
}

func TestEchoSettingsUpdateRequiresUserID(t *testing.T) {
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, &fakeEchoRepo{})
	enabled := true
	_, err := svc.UpdateSettings(context.Background(), UpdateEchoSettingsInput{
		ExpectedRevision: 0, Enabled: &enabled,
	})
	if !errors.Is(err, ErrInvalidEchoSettings) {
		t.Fatalf("expected ErrInvalidEchoSettings, got %v", err)
	}
}

func TestEchoSettingsGetRequiresUserID(t *testing.T) {
	svc := newEchoSvc(&fakeEchoSettingsRepo{}, &fakeEchoRepo{})
	_, err := svc.GetSettings(context.Background(), uuid.Nil)
	if !errors.Is(err, ErrInvalidEchoSettings) {
		t.Fatalf("expected ErrInvalidEchoSettings, got %v", err)
	}
}

func TestEchoSettingsUpdateRepositoryConflictMapsToServiceConflict(t *testing.T) {
	repo := &fakeEchoSettingsRepo{createConflict: true}
	svc := newEchoSvc(repo, &fakeEchoRepo{})
	enabled := true

	_, err := svc.UpdateSettings(context.Background(), UpdateEchoSettingsInput{
		UserID: uuid.New(), ExpectedRevision: 0, Enabled: &enabled,
	})
	if !errors.Is(err, ErrEchoSettingsConflict) {
		t.Fatalf("expected ErrEchoSettingsConflict, got %v", err)
	}
}
