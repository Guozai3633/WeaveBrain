package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/service"
	"weavebrain/pkg/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// r11Env bundles the real-PostgreSQL harness shared by the R11 echo tests: a
// connected pool for direct-SQL assertions plus the assembled gin engine wired
// through the real service/repository stack (mirroring the R10 harness).
type r11Env struct {
	ctx    context.Context
	pool   *pgxpool.Pool
	engine http.Handler
}

func newR11Env(t *testing.T) *r11Env {
	t.Helper()
	databaseURL := os.Getenv("WEAVEBRAIN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WEAVEBRAIN_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping test database: %v", err)
	}

	store := repository.NewFromPool(pool)
	key := []byte("0123456789abcdef0123456789abcdef")
	tokenConfig := auth.TokenConfig{
		Secret: []byte("r11-echo-secret"),
		Issuer: "weavebrain",
		Expiry: time.Hour,
	}
	server := NewServer("0", service.New(store, key), tokenConfig, "mock", nil, nil, key)
	return &r11Env{ctx: ctx, pool: pool, engine: server.engine}
}

func (e *r11Env) close() {
	e.pool.Close()
}

func (e *r11Env) registerUser(t *testing.T, label string) (string, uuid.UUID) {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"provider":    "r11_test",
		"provider_id": label + "-" + uuid.NewString(),
	})
	recorder := performR3Request(
		e.engine,
		http.MethodPost,
		"/api/v1/auth/register",
		body,
		"",
		"",
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("register: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var registered map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &registered); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	token, _ := registered["token"].(string)
	userObj, _ := registered["user"].(map[string]any)
	userIDText, _ := userObj["id"].(string)
	userID, err := uuid.Parse(userIDText)
	if token == "" || err != nil {
		t.Fatalf("register response incomplete: %s", recorder.Body.String())
	}
	return token, userID
}

func (e *r11Env) createTextCapture(t *testing.T, token, text string) uuid.UUID {
	t.Helper()
	captureID := uuid.New()
	recorder := performR3Request(
		e.engine,
		http.MethodPost,
		"/api/v3/captures",
		r3CaptureBody(captureID, text),
		token,
		captureID.String(),
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create capture: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	return captureID
}

func (e *r11Env) patchEchoSettings(t *testing.T, token string, body map[string]any) (int, string) {
	t.Helper()
	payload, _ := json.Marshal(body)
	recorder := performR3Request(
		e.engine,
		http.MethodPatch,
		"/api/v3/users/me/echo-settings",
		payload,
		token,
		"",
	)
	return recorder.Code, recorder.Body.String()
}

func (e *r11Env) enableDailyEcho(t *testing.T, token string) {
	t.Helper()
	status, body := e.patchEchoSettings(t, token, map[string]any{
		"expected_revision": 0,
		"enabled":           true,
		"cadence":           "daily",
	})
	if status != http.StatusOK {
		t.Fatalf("enable daily echo: status=%d body=%s", status, body)
	}
}

func (e *r11Env) getSettings(t *testing.T, token string) (int, string) {
	t.Helper()
	recorder := performR3Request(
		e.engine,
		http.MethodGet,
		"/api/v3/users/me/echo-settings",
		nil,
		token,
		"",
	)
	return recorder.Code, recorder.Body.String()
}

func (e *r11Env) getCurrentEcho(t *testing.T, token string) (int, EchoCurrentResponse) {
	t.Helper()
	recorder := performR3Request(
		e.engine,
		http.MethodGet,
		"/api/v3/echoes/current",
		nil,
		token,
		"",
	)
	var response EchoCurrentResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode current echo: %v body=%s", err, recorder.Body.String())
		}
	}
	return recorder.Code, response
}

func (e *r11Env) sendFeedback(t *testing.T, token string, echoID uuid.UUID, verdict string) (int, string) {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{"verdict": verdict})
	recorder := performR3Request(
		e.engine,
		http.MethodPost,
		"/api/v3/echoes/"+echoID.String()+"/feedback",
		payload,
		token,
		"",
	)
	return recorder.Code, recorder.Body.String()
}

func (e *r11Env) countEchoes(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	var count int
	if err := e.pool.QueryRow(
		e.ctx,
		"SELECT count(*) FROM user_echoes WHERE user_id = $1",
		userID,
	).Scan(&count); err != nil {
		t.Fatalf("count user_echoes: %v", err)
	}
	return count
}

// backdateUserEchoes pushes the user's echo rows N days into the past. The
// server cadence gate anchors on the most recent echo row's created_at, so this
// opens the next cadence window without any clock manipulation in the service.
func (e *r11Env) backdateUserEchoes(t *testing.T, userID uuid.UUID, days int) {
	t.Helper()
	if _, err := e.pool.Exec(
		e.ctx,
		"UPDATE user_echoes SET created_at = created_at - ($1 * interval '1 day'), updated_at = now() WHERE user_id = $2",
		days,
		userID,
	); err != nil {
		t.Fatalf("backdate user_echoes: %v", err)
	}
}

func (e *r11Env) captureCount(t *testing.T, userID uuid.UUID) int {
	t.Helper()
	var count int
	if err := e.pool.QueryRow(
		e.ctx,
		"SELECT count(*) FROM captures WHERE user_id = $1 AND deleted_at IS NULL",
		userID,
	).Scan(&count); err != nil {
		t.Fatalf("count captures: %v", err)
	}
	return count
}

// decodeEchoError unmarshals a V3 error body, failing the test on any other
// shape so a wrong success/failure path surfaces loudly.
func decodeEchoError(t *testing.T, body string) V3ErrorResponse {
	t.Helper()
	var response V3ErrorResponse
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		t.Fatalf("decode V3 error: %v body=%s", err, body)
	}
	return response
}

// TestR11EchoSettingsGatingAndFeedback is the G9 echo evidence on a real
// PostgreSQL database:
//
//  1. Echo is off by default: GET /echoes/current returns enabled=false and the
//     server creates no echo row.
//  2. Enabling daily cadence materializes a server-side settings row (revision
//     CAS) and the next read offers exactly one echo — repeated reads reuse the
//     same open echo (stable single-row discipline) instead of creating more.
//  3. A done verdict resolves the echo; the next read is an explicit off-period
//     (no new row) until the cadence window passes, and re-feedback on the
//     resolved echo is a 409 VERSION_CONFLICT.
//  4. Once the window opens (row back-dated past the one-day cadence), the next
//     echo is offered on a different capture: the answered capture is inside
//     its 14-day cooldown and the other card rotates in.
func TestR11EchoSettingsGatingAndFeedback(t *testing.T) {
	env := newR11Env(t)
	defer env.close()

	token, userID := env.registerUser(t, "r11-gating")

	// --- Default: echo disabled → explicit empty payload, zero rows. ---
	status, current := env.getCurrentEcho(t, token)
	if status != http.StatusOK {
		t.Fatalf("current (disabled): status=%d", status)
	}
	if current.Enabled {
		t.Fatal("current (disabled) must report enabled=false")
	}
	if current.Echo != nil {
		t.Fatalf("current (disabled) must omit echo, got %+v", current.Echo)
	}
	if got := env.countEchoes(t, userID); got != 0 {
		t.Fatalf("user_echoes count=%d while disabled, want 0", got)
	}

	// --- Enable daily cadence (revision 0 → 1). ---
	env.enableDailyEcho(t, token)
	status, settingsBody := env.getSettings(t, token)
	if status != http.StatusOK {
		t.Fatalf("get echo settings after enable: status=%d", status)
	}
	var settingsResp EchoSettingsResponse
	if err := json.Unmarshal([]byte(settingsBody), &settingsResp); err != nil {
		t.Fatalf("decode echo settings: %v", err)
	}
	if settingsResp.Settings == nil || !settingsResp.Settings.Enabled ||
		settingsResp.Settings.Cadence != "daily" || settingsResp.Settings.Revision != 1 {
		t.Fatalf("echo settings after enable = %+v, want enabled daily revision 1", settingsResp.Settings)
	}

	// --- Seed two distinct text captures (both become ready memory cards). ---
	first := env.createTextCapture(t, token, "R11 gating memory one: an early idea about echoplex")
	second := env.createTextCapture(t, token, "R11 gating memory two: a later note about quiet hours")
	if got := env.captureCount(t, userID); got != 2 {
		t.Fatalf("captures count=%d, want 2", got)
	}

	// --- First read offers the oldest capture with reason first_echo. ---
	status, current = env.getCurrentEcho(t, token)
	if status != http.StatusOK {
		t.Fatalf("current (first): status=%d", status)
	}
	if current.Echo == nil {
		t.Fatalf("current (first) must offer an echo, got %+v", current)
	}
	echoOne := *current.Echo
	if echoOne.Status != "open" {
		t.Fatalf("first echo status=%q, want open", echoOne.Status)
	}
	if echoOne.Reason.Code != "first_echo" {
		t.Fatalf("first echo reason=%q, want first_echo", echoOne.Reason.Code)
	}
	if echoOne.Memory.CaptureID != first && echoOne.Memory.CaptureID != second {
		t.Fatalf("first echo capture=%s, want one of the seeded captures", echoOne.Memory.CaptureID)
	}
	firstCapture := echoOne.Memory.CaptureID
	otherCapture := second
	if firstCapture == second {
		otherCapture = first
	}
	if got := env.countEchoes(t, userID); got != 1 {
		t.Fatalf("user_echoes count=%d after first read, want 1", got)
	}

	// --- Repeated reads reuse the same open echo: still exactly one row. ---
	status, current = env.getCurrentEcho(t, token)
	if status != http.StatusOK || current.Echo == nil {
		t.Fatalf("current (replay) must reuse the open echo: status=%d echo=%v", status, current.Echo)
	}
	if current.Echo.ID != echoOne.ID {
		t.Fatalf("replayed echo id=%s, want stable %s", current.Echo.ID, echoOne.ID)
	}
	if got := env.countEchoes(t, userID); got != 1 {
		t.Fatalf("user_echoes count=%d after replay, want 1 (stable single-row)", got)
	}

	// --- Feedback done resolves the echo. ---
	status, feedbackBody := env.sendFeedback(t, token, echoOne.ID, "done")
	if status != http.StatusOK {
		t.Fatalf("feedback done: status=%d body=%s", status, feedbackBody)
	}
	var feedbackResult EchoFeedbackResponse
	if err := json.Unmarshal([]byte(feedbackBody), &feedbackResult); err != nil {
		t.Fatalf("decode feedback response: %v", err)
	}
	if feedbackResult.Status != "done" || feedbackResult.EchoID != echoOne.ID {
		t.Fatalf("feedback result = %+v, want done on %s", feedbackResult, echoOne.ID)
	}
	if got := env.countEchoes(t, userID); got != 1 {
		t.Fatalf("user_echoes count=%d after feedback, want 1", got)
	}

	// --- Next read is an explicit off-period: no new row until cadence passes. ---
	status, current = env.getCurrentEcho(t, token)
	if status != http.StatusOK {
		t.Fatalf("current (off-period): status=%d", status)
	}
	if !current.Enabled || current.Echo != nil {
		t.Fatalf("current (off-period) = %+v, want enabled=true with no echo", current)
	}
	if current.NextDueAt == nil {
		t.Fatal("current (off-period) must include next_due_at")
	}
	if got := env.countEchoes(t, userID); got != 1 {
		t.Fatalf("user_echoes count=%d after off-period read, want 1", got)
	}

	// --- Re-feedback on the resolved echo is a conflict, and a foreign echo 404. ---
	status, feedbackBody = env.sendFeedback(t, token, echoOne.ID, "done")
	if status != http.StatusConflict {
		t.Fatalf("re-feedback done: status=%d body=%s, want 409", status, feedbackBody)
	}
	if code := decodeEchoError(t, feedbackBody).Code; code != V3ErrorVersionConflict {
		t.Fatalf("re-feedback code=%q, want VERSION_CONFLICT", code)
	}
	status, feedbackBody = env.sendFeedback(t, token, uuid.New(), "done")
	if status != http.StatusNotFound {
		t.Fatalf("feedback on missing echo: status=%d body=%s, want 404", status, feedbackBody)
	}
	if code := decodeEchoError(t, feedbackBody).Code; code != V3ErrorNotFound {
		t.Fatalf("missing-echo feedback code=%q, want NOT_FOUND", code)
	}

	// --- Cadence window opens: next echo rotates to the other capture. ---
	env.backdateUserEchoes(t, userID, 2)
	status, current = env.getCurrentEcho(t, token)
	if status != http.StatusOK || current.Echo == nil {
		t.Fatalf("current (after window): status=%d echo=%v", status, current.Echo)
	}
	echoTwo := *current.Echo
	if echoTwo.ID == echoOne.ID {
		t.Fatal("second echo must be a new row, not the resolved one")
	}
	if echoTwo.Status != "open" {
		t.Fatalf("second echo status=%q, want open", echoTwo.Status)
	}
	if echoTwo.Memory.CaptureID != otherCapture {
		t.Fatalf("second echo capture=%s, want the other seeded capture %s (answered one is in cooldown)", echoTwo.Memory.CaptureID, otherCapture)
	}
	if echoTwo.Reason.Code != "oldest" {
		t.Fatalf("second echo reason=%q, want oldest", echoTwo.Reason.Code)
	}
	if got := env.countEchoes(t, userID); got != 2 {
		t.Fatalf("user_echoes count=%d after second echo, want 2", got)
	}
}

// TestR11EchoNotRelevantAndCrossUser exercises the longer not_relevant cooldown
// and user isolation on a real database:
//
//  1. A not_relevant verdict blocks re-offering the same capture for 90 days —
//     longer than the 14-day window of other verdicts. Back-dating the echo by
//     20 days opens the cadence window yet the answered capture is still
//     excluded, so the other card is offered instead.
//  2. A second user echoes their own captures immediately and is completely
//     unaffected by the first user's echo rows (every read/row is user-scoped).
func TestR11EchoNotRelevantAndCrossUser(t *testing.T) {
	env := newR11Env(t)
	defer env.close()

	// --- User A: two captures, not_relevant on the first echo. ---
	userA, aID := env.registerUser(t, "r11-not-relevant")
	env.enableDailyEcho(t, userA)
	firstA := env.createTextCapture(t, userA, "R11 not relevant memory one: pinned-free alpha thought")
	secondA := env.createTextCapture(t, userA, "R11 not relevant memory two: unrelated beta plan")

	status, current := env.getCurrentEcho(t, userA)
	if status != http.StatusOK || current.Echo == nil {
		t.Fatalf("user A current (first): status=%d echo=%v", status, current.Echo)
	}
	echoA := *current.Echo
	answeredA := echoA.Memory.CaptureID
	otherA := secondA
	if answeredA == secondA {
		otherA = firstA
	}
	status, body := env.sendFeedback(t, userA, echoA.ID, "not_relevant")
	if status != http.StatusOK {
		t.Fatalf("user A feedback not_relevant: status=%d body=%s", status, body)
	}
	if got := env.countEchoes(t, aID); got != 1 {
		t.Fatalf("user A user_echoes count=%d, want 1", got)
	}

	// --- Back-date 20 days: cadence gate opens (daily) but the not_relevant
	// exclusion (90 days) still blocks the answered capture. ---
	env.backdateUserEchoes(t, aID, 20)
	status, current = env.getCurrentEcho(t, userA)
	if status != http.StatusOK || current.Echo == nil {
		t.Fatalf("user A current (rotated): status=%d echo=%v", status, current.Echo)
	}
	echoA2 := *current.Echo
	if echoA2.Memory.CaptureID != otherA {
		t.Fatalf(
			"user A second echo capture=%s, want %s (not_relevant capture is inside its 90-day exclusion)",
			echoA2.Memory.CaptureID,
			otherA,
		)
	}
	if got := env.countEchoes(t, aID); got != 2 {
		t.Fatalf("user A user_echoes count=%d after rotation, want 2", got)
	}

	// --- User B: an independent flow is unaffected by A's echoes. ---
	userB, bID := env.registerUser(t, "r11-cross-user")
	env.enableDailyEcho(t, userB)
	onlyB := env.createTextCapture(t, userB, "R11 user B lone capture, should be immediately echoable")
	status, current = env.getCurrentEcho(t, userB)
	if status != http.StatusOK || current.Echo == nil {
		t.Fatalf("user B current: status=%d echo=%v", status, current.Echo)
	}
	echoB := *current.Echo
	if echoB.Memory.CaptureID != onlyB {
		t.Fatalf("user B echo capture=%s, want its own capture %s", echoB.Memory.CaptureID, onlyB)
	}
	if echoB.Reason.Code != "first_echo" {
		t.Fatalf("user B echo reason=%q, want first_echo", echoB.Reason.Code)
	}
	if got := env.countEchoes(t, bID); got != 1 {
		t.Fatalf("user B user_echoes count=%d, want 1", got)
	}
	if got := env.countEchoes(t, aID); got != 2 {
		t.Fatalf("user A user_echoes count=%d after B echoed, want still 2 (isolation)", got)
	}
}
