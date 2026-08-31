package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/entity"

	"github.com/google/uuid"
)

type retryRecord struct {
	id      int64
	backoff time.Duration
	reason  string
}

type reorganizeRecord struct {
	userID uuid.UUID
	policy []byte
}

type fakeOutboxRepo struct {
	events        []*entity.CaptureOutbox
	ready         []int64
	failed        map[int64]string
	retry         []retryRecord
	cancelled     []uuid.UUID
	claimErr      error
	pendingCount  int64
	reorganized   []reorganizeRecord
	reorganizeErr error
}

func (f *fakeOutboxRepo) ClaimDue(context.Context, int, time.Duration) ([]*entity.CaptureOutbox, error) {
	if f.claimErr != nil {
		return nil, f.claimErr
	}
	events := f.events
	f.events = nil
	return events, nil
}

func (f *fakeOutboxRepo) MarkReady(_ context.Context, id int64) error {
	f.ready = append(f.ready, id)
	return nil
}

func (f *fakeOutboxRepo) MarkFailed(_ context.Context, id int64, reason string) error {
	if f.failed == nil {
		f.failed = make(map[int64]string)
	}
	f.failed[id] = reason
	return nil
}

func (f *fakeOutboxRepo) MarkRetryWait(_ context.Context, id int64, backoff time.Duration, reason string) error {
	f.retry = append(f.retry, retryRecord{id: id, backoff: backoff, reason: reason})
	return nil
}

func (f *fakeOutboxRepo) CancelByUser(_ context.Context, userID uuid.UUID) (int64, error) {
	f.cancelled = append(f.cancelled, userID)
	return 1, nil
}

func (f *fakeOutboxRepo) CountQueued(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (f *fakeOutboxRepo) CountPendingReorganize(context.Context, uuid.UUID) (int64, error) {
	return f.pendingCount, nil
}

func (f *fakeOutboxRepo) ReorganizeByUser(_ context.Context, userID uuid.UUID, policy []byte) (int64, error) {
	if f.reorganizeErr != nil {
		return 0, f.reorganizeErr
	}
	f.reorganized = append(f.reorganized, reorganizeRecord{
		userID: userID,
		policy: append([]byte(nil), policy...),
	})
	return int64(len(f.reorganized)), nil
}

type workerCaptureRepo struct {
	aggregate *entity.CaptureAggregate
	err       error
}

func (r *workerCaptureRepo) Create(context.Context, *entity.Capture, *entity.MemoryCard, *entity.EnrichmentRevision, *entity.PolicySnapshot) (bool, error) {
	panic("not used in worker test")
}

func (r *workerCaptureRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*entity.CaptureAggregate, error) {
	return r.aggregate, r.err
}

func (r *workerCaptureRepo) UpdateCardTitle(context.Context, uuid.UUID, uuid.UUID, string) error {
	panic("not used in worker test")
}

type countingPipeline struct {
	mu          sync.Mutex
	organize    int
	embed       int
	relate      int
	recap       int
	organizeErr error
	embedErr    error
	relateErr   error
	recapErr    error
}

func (p *countingPipeline) counts() (int, int, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.organize, p.embed, p.relate, p.recap
}

func (p *countingPipeline) Organize(context.Context, *entity.Capture, *entity.MemoryCard) error {
	p.mu.Lock()
	p.organize++
	p.mu.Unlock()
	return p.organizeErr
}

func (p *countingPipeline) Embed(context.Context, *entity.Capture, *entity.MemoryCard) error {
	p.mu.Lock()
	p.embed++
	p.mu.Unlock()
	return p.embedErr
}

func (p *countingPipeline) Relate(context.Context, *entity.Capture, *entity.MemoryCard) error {
	p.mu.Lock()
	p.relate++
	p.mu.Unlock()
	return p.relateErr
}

func (p *countingPipeline) Recap(context.Context, *entity.Capture, *entity.MemoryCard) error {
	p.mu.Lock()
	p.recap++
	p.mu.Unlock()
	return p.recapErr
}

func newWorkerTestEvent(id int64, snapshot *entity.PolicySnapshot, payload map[string]any, attempts int) *entity.CaptureOutbox {
	if snapshot == nil {
		snapshot = entity.DefaultPolicySnapshot()
	}
	if payload == nil {
		payload = map[string]any{"privacy_mode": "cloud_allowed"}
	}
	return &entity.CaptureOutbox{
		ID:             id,
		UserID:         uuid.New(),
		CaptureID:      uuid.New(),
		EventType:      "capture.created",
		Payload:        payload,
		PolicySnapshot: snapshot,
		Status:         "processing",
		AttemptCount:   attempts,
		NextRunAt:      time.Now().UTC(),
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
}

func workerWithConfig(outbox *fakeOutboxRepo, captures *workerCaptureRepo, pipeline EnrichmentPipeline) *OutboxWorker {
	cfg := OutboxWorkerConfig{
		ClaimBatch:   10,
		PollInterval: time.Hour, // never fires in tests; we call processDue directly
		MaxAttempts:  3,
		RetryBase:    time.Second,
		RetryMax:     time.Minute,
	}
	return NewOutboxWorker(outbox, captures, pipeline, cfg)
}

func TestOutboxWorkerAIDisabledMarksReadyWithoutPipelineCalls(t *testing.T) {
	outbox := &fakeOutboxRepo{events: []*entity.CaptureOutbox{
		newWorkerTestEvent(1, entity.DefaultPolicySnapshot(), nil, 1),
	}}
	pipeline := &countingPipeline{}
	captures := &workerCaptureRepo{}
	worker := workerWithConfig(outbox, captures, pipeline)

	if err := worker.processDue(context.Background()); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if len(outbox.ready) != 1 || outbox.ready[0] != 1 {
		t.Fatalf("expected event 1 marked ready, got %#v", outbox.ready)
	}
	if o, e, r, c := pipeline.counts(); o+e+r+c != 0 {
		t.Fatalf("AI disabled must not call the pipeline, got organize=%d embed=%d relate=%d recap=%d", o, e, r, c)
	}
	if len(outbox.failed) != 0 || len(outbox.retry) != 0 {
		t.Fatalf("expected no failure/retry, got failed=%#v retry=%#v", outbox.failed, outbox.retry)
	}
}

func TestOutboxWorkerAIDisabledKeepsFallbackRevision(t *testing.T) {
	// A capture created while AI organizing is off still records its local
	// fallback revision atomically at creation time; the worker's no-op
	// pipeline never overwrites it with an ai revision.
	captures := newMemoryCaptureRepository()
	userID := uuid.New()
	captureID := uuid.New()
	if _, err := NewCaptureService(captures, nil).Create(context.Background(), userID, CreateCaptureInput{
		ID:            captureID,
		Kind:          entity.CaptureKindText,
		Text:          "离线也能先记住",
		ClientVersion: 1,
	}); err != nil {
		t.Fatalf("create capture: %v", err)
	}
	key := captureKey{userID: userID, captureID: captureID}
	revs := captures.revisions[key]
	if len(revs) != 1 {
		t.Fatalf("expected exactly one fallback revision after creation, got %d", len(revs))
	}
	if revs[0].Source != entity.EnrichmentSourceFallback {
		t.Fatalf("expected fallback source, got %q", revs[0].Source)
	}

	outbox := &fakeOutboxRepo{events: []*entity.CaptureOutbox{
		newWorkerTestEvent(1, entity.DefaultPolicySnapshot(), nil, 1),
	}}
	pipeline := &countingPipeline{}
	worker := workerWithConfig(outbox, &workerCaptureRepo{}, pipeline)
	if err := worker.processDue(context.Background()); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if o, e, r, c := pipeline.counts(); o+e+r+c != 0 {
		t.Fatalf("AI disabled must not call the pipeline, got %d", o+e+r+c)
	}
	if got := len(captures.revisions[key]); got != 1 {
		t.Fatalf("worker must not append an ai revision, got %d revisions", got)
	}
}

func TestOutboxWorkerPrivacyNoAISkipsPipelineEvenWhenAIEnabled(t *testing.T) {
	outbox := &fakeOutboxRepo{events: []*entity.CaptureOutbox{
		newWorkerTestEvent(1, &entity.PolicySnapshot{AIMemoryEnabled: true},
			map[string]any{"privacy_mode": "no_ai"}, 1),
	}}
	pipeline := &countingPipeline{}
	worker := workerWithConfig(outbox, &workerCaptureRepo{}, pipeline)

	if err := worker.processDue(context.Background()); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if len(outbox.ready) != 1 {
		t.Fatalf("expected no_ai event marked ready, got %#v", outbox.ready)
	}
	if o, e, r, c := pipeline.counts(); o+e+r+c != 0 {
		t.Fatalf("no_ai must not call the pipeline, got %d", o+e+r+c)
	}
}

func TestOutboxWorkerAIEnabledRunsFullPipelineThenReady(t *testing.T) {
	outbox := &fakeOutboxRepo{events: []*entity.CaptureOutbox{
		newWorkerTestEvent(1, &entity.PolicySnapshot{AIMemoryEnabled: true}, nil, 1),
	}}
	pipeline := &countingPipeline{}
	captures := &workerCaptureRepo{aggregate: &entity.CaptureAggregate{
		Capture:    &entity.Capture{},
		MemoryCard: &entity.MemoryCard{},
	}}
	worker := workerWithConfig(outbox, captures, pipeline)

	if err := worker.processDue(context.Background()); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if o, e, r, c := pipeline.counts(); o != 1 || e != 1 || r != 1 || c != 1 {
		t.Fatalf("expected pipeline to run once each, got %d/%d/%d/%d", o, e, r, c)
	}
	if len(outbox.ready) != 1 {
		t.Fatalf("expected event marked ready after enrichment, got %#v", outbox.ready)
	}
	if len(outbox.failed) != 0 || len(outbox.retry) != 0 {
		t.Fatalf("expected no failure/retry, got failed=%#v retry=%#v", outbox.failed, outbox.retry)
	}
}

func TestOutboxWorkerPipelineErrorSchedulesRetryWithBackoff(t *testing.T) {
	outbox := &fakeOutboxRepo{events: []*entity.CaptureOutbox{
		newWorkerTestEvent(1, &entity.PolicySnapshot{AIMemoryEnabled: true}, nil, 1),
	}}
	pipeline := &countingPipeline{organizeErr: errWorkerTest("llm down")}
	worker := workerWithConfig(outbox, &workerCaptureRepo{aggregate: &entity.CaptureAggregate{}}, pipeline)

	if err := worker.processDue(context.Background()); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if len(outbox.retry) != 1 || outbox.retry[0].id != 1 {
		t.Fatalf("expected event 1 scheduled for retry, got %#v", outbox.retry)
	}
	if outbox.retry[0].backoff < time.Second {
		t.Fatalf("expected exponential backoff >= base, got %v", outbox.retry[0].backoff)
	}
	if len(outbox.ready) != 0 || len(outbox.failed) != 0 {
		t.Fatalf("retry must not mark ready/failed, got ready=%#v failed=%#v", outbox.ready, outbox.failed)
	}
}

func TestOutboxWorkerMaxAttemptsMarksFailedTerminal(t *testing.T) {
	outbox := &fakeOutboxRepo{events: []*entity.CaptureOutbox{
		newWorkerTestEvent(1, &entity.PolicySnapshot{AIMemoryEnabled: true}, nil, 3), // == MaxAttempts
	}}
	pipeline := &countingPipeline{embedErr: errWorkerTest("persistent failure")}
	worker := workerWithConfig(outbox, &workerCaptureRepo{aggregate: &entity.CaptureAggregate{}}, pipeline)

	if err := worker.processDue(context.Background()); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if len(outbox.failed) != 1 || outbox.failed[1] == "" {
		t.Fatalf("expected terminal failure, got %#v", outbox.failed)
	}
	if len(outbox.retry) != 0 {
		t.Fatalf("max attempts must not retry again, got %#v", outbox.retry)
	}
}

func TestOutboxWorkerCaptureNotFoundMarksFailedWithoutRetry(t *testing.T) {
	outbox := &fakeOutboxRepo{events: []*entity.CaptureOutbox{
		newWorkerTestEvent(1, &entity.PolicySnapshot{AIMemoryEnabled: true}, nil, 1),
	}}
	worker := workerWithConfig(outbox, &workerCaptureRepo{err: repository.ErrCaptureNotFound}, &countingPipeline{})

	if err := worker.processDue(context.Background()); err != nil {
		t.Fatalf("processDue: %v", err)
	}
	if len(outbox.failed) != 1 {
		t.Fatalf("capture-not-found must fail immediately, got %#v", outbox.failed)
	}
	if len(outbox.retry) != 0 {
		t.Fatalf("capture-not-found must not retry, got %#v", outbox.retry)
	}
}

func TestOutboxWorkerRetryBackoffIsExponentialAndCapped(t *testing.T) {
	worker := workerWithConfig(&fakeOutboxRepo{}, &workerCaptureRepo{}, nil)
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, time.Second},     // base
		{2, 2 * time.Second}, // doubled
		{3, 4 * time.Second}, // doubled
		{4, 8 * time.Second}, // doubled
		{10, time.Minute},    // capped at RetryMax
	}
	for _, c := range cases {
		if got := worker.retryBackoff(c.attempt); got != c.want {
			t.Fatalf("retryBackoff(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestOutboxWorkerStartStopRunsCleanly(t *testing.T) {
	outbox := &fakeOutboxRepo{}
	worker := workerWithConfig(outbox, &workerCaptureRepo{}, nil)
	worker.Start()
	worker.Stop()
	// A second Start/Stop must be a no-op (guarded by done != nil).
	worker.Start()
	worker.Stop()
}

type errWorkerTest string

func (e errWorkerTest) Error() string { return string(e) }
