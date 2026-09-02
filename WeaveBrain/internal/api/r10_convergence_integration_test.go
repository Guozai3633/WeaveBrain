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

// TestR10CaptureConvergenceAndWorkflowGuard is the G8 convergence evidence on a
// real PostgreSQL database:
//
//  1. A phone capture (device A) and a Web review client converge on exactly one
//     server-side Capture — a second device pushing the identical canonical
//     payload for the same capture UUID is an idempotent 200 replay, never a
//     second row ("同一 Capture 在手机/Web 最终一致").
//  2. Conflict handling stays server-authoritative: a same-UUID push with
//     different content returns 409 IDEMPOTENCY_CONFLICT and creates no row.
//  3. The reserved /api/v3/workflows namespace returns 501 FEATURE_NOT_ENABLED
//     and creates no capture_outbox/background work ("误调用工作流接口不创建任务
//     或副作用"); GET /api/v3/capabilities advertises workflow_*=false so well
//     behaved clients never call the namespace.
func TestR10CaptureConvergenceAndWorkflowGuard(t *testing.T) {
	databaseURL := os.Getenv("WEAVEBRAIN_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("WEAVEBRAIN_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}

	store := repository.NewFromPool(pool)
	key := []byte("0123456789abcdef0123456789abcdef")
	tokenConfig := auth.TokenConfig{
		Secret: []byte("r10-convergence-secret"),
		Issuer: "weavebrain",
		Expiry: time.Hour,
	}
	server := NewServer("0", service.New(store, key), tokenConfig, "mock", nil, nil, key)
	engine := server.engine

	suffix := uuid.NewString()
	body, _ := json.Marshal(map[string]any{
		"provider":    "r10_test",
		"provider_id": "device-web-" + suffix,
	})
	register := performR3Request(
		engine,
		http.MethodPost,
		"/api/v1/auth/register",
		body,
		"",
		"",
	)
	if register.Code != http.StatusCreated {
		t.Fatalf("register: status=%d body=%s", register.Code, register.Body.String())
	}
	var registered map[string]any
	if err := json.Unmarshal(register.Body.Bytes(), &registered); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	token, _ := registered["token"].(string)
	userObj, _ := registered["user"].(map[string]any)
	userIDText, _ := userObj["id"].(string)
	userID, err := uuid.Parse(userIDText)
	if token == "" || err != nil {
		t.Fatalf("register response incomplete: %s", register.Body.String())
	}

	// --- Convergence: device A and a Web replay produce one server row. ---
	captureID := uuid.New()
	captureText := "R10 cross-device convergence capture"
	canonicalBody := r3CaptureBody(captureID, captureText)

	countCaptures := func() int {
		t.Helper()
		var count int
		if err := pool.QueryRow(
			ctx,
			"SELECT count(*) FROM captures WHERE user_id = $1 AND id = $2",
			userID,
			captureID,
		).Scan(&count); err != nil {
			t.Fatalf("count captures: %v", err)
		}
		return count
	}
	countOutbox := func() int {
		t.Helper()
		var count int
		if err := pool.QueryRow(
			ctx,
			"SELECT count(*) FROM capture_outbox WHERE user_id = $1",
			userID,
		).Scan(&count); err != nil {
			t.Fatalf("count capture_outbox: %v", err)
		}
		return count
	}

	deviceA := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/captures",
		canonicalBody,
		token,
		captureID.String(),
	)
	if deviceA.Code != http.StatusCreated {
		t.Fatalf("device A create: status=%d body=%s", deviceA.Code, deviceA.Body.String())
	}

	// The Web client restores/replays the same queued capture (identical
	// canonical payload for the same UUID): idempotent 200, not a duplicate.
	webReplay := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/captures",
		canonicalBody,
		token,
		captureID.String(),
	)
	if webReplay.Code != http.StatusOK {
		t.Fatalf("web replay: status=%d body=%s", webReplay.Code, webReplay.Body.String())
	}

	if got := countCaptures(); got != 1 {
		t.Fatalf("captures count=%d after device A + web replay, want 1", got)
	}
	for table, query := range map[string]string{
		"memory_cards":   "SELECT count(*) FROM memory_cards WHERE user_id = $1 AND capture_id = $2",
		"capture_outbox": "SELECT count(*) FROM capture_outbox WHERE user_id = $1 AND capture_id = $2",
	} {
		var count int
		if err := pool.QueryRow(ctx, query, userID, captureID).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s count=%d, want 1", table, count)
		}
	}

	// Both clients read the same single record through the API.
	read := performR3Request(
		engine,
		http.MethodGet,
		"/api/v3/captures/"+captureID.String(),
		nil,
		token,
		"",
	)
	if read.Code != http.StatusOK {
		t.Fatalf("read converged capture: status=%d body=%s", read.Code, read.Body.String())
	}

	// --- Conflict handling: same UUID, different content → 409, still one row. ---
	conflict := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/captures",
		r3CaptureBody(captureID, "conflicting edit"),
		token,
		captureID.String(),
	)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("idempotency conflict: status=%d body=%s", conflict.Code, conflict.Body.String())
	}
	if got := countCaptures(); got != 1 {
		t.Fatalf("captures count=%d after conflict, want 1", got)
	}

	// --- Workflow guard: 501 and no side effects. ---
	outboxBefore := countOutbox()

	capabilities := performR3Request(
		engine,
		http.MethodGet,
		"/api/v3/capabilities",
		nil,
		"",
		"",
	)
	if capabilities.Code != http.StatusOK {
		t.Fatalf("capabilities: status=%d body=%s", capabilities.Code, capabilities.Body.String())
	}
	var caps V3CapabilitiesResponse
	if err := json.Unmarshal(capabilities.Body.Bytes(), &caps); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}
	if caps.WorkflowDesigner || caps.WorkflowExecution {
		t.Fatal("capabilities must report workflow_designer=false and workflow_execution=false")
	}

	workflowBody, _ := json.Marshal(map[string]any{
		"name":      "should never run",
		"trigger":   "manual",
		"execution": true,
		"designer":  true,
	})
	workflowCall := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/workflows",
		workflowBody,
		token,
		"",
	)
	if workflowCall.Code != http.StatusNotImplemented {
		t.Fatalf("workflow guard: status=%d body=%s", workflowCall.Code, workflowCall.Body.String())
	}
	var workflowErr V3ErrorResponse
	if err := json.Unmarshal(workflowCall.Body.Bytes(), &workflowErr); err != nil {
		t.Fatalf("decode workflow guard response: %v", err)
	}
	if workflowErr.Code != V3ErrorFeatureNotEnabled {
		t.Fatalf("workflow guard code=%q, want %q", workflowErr.Code, V3ErrorFeatureNotEnabled)
	}

	if got := countOutbox(); got != outboxBefore {
		t.Fatalf(
			"capture_outbox count changed after workflow 501 (%d → %d); workflow call must create no side effects",
			outboxBefore,
			got,
		)
	}
}
