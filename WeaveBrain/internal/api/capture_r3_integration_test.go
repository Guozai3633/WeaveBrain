package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"weavebrain/internal/db/repository"
	"weavebrain/internal/service"
	"weavebrain/pkg/auth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestR3CaptureFullHTTPPostgresAcceptance(t *testing.T) {
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
		Secret: []byte("r3-acceptance-secret"),
		Issuer: "weavebrain",
		Expiry: time.Hour,
	}
	server := NewServer("0", service.New(store, key), tokenConfig, "mock", nil, nil, key)
	engine := server.engine

	register := func(providerID string) (string, uuid.UUID) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"provider":    "r3_test",
			"provider_id": providerID,
		})
		recorder := performR3Request(
			engine,
			http.MethodPost,
			"/api/v1/auth/register",
			body,
			"",
			"",
		)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("register %s: status=%d body=%s", providerID, recorder.Code, recorder.Body.String())
		}

		var response map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode register response: %v", err)
		}
		token, _ := response["token"].(string)
		user, _ := response["user"].(map[string]any)
		userIDText, _ := user["id"].(string)
		userID, err := uuid.Parse(userIDText)
		if err != nil || token == "" || userID == uuid.Nil {
			t.Fatalf("register response incomplete: %s", recorder.Body.String())
		}
		return token, userID
	}

	suffix := uuid.NewString()
	ownerToken, ownerID := register("owner-" + suffix)
	otherToken, _ := register("other-" + suffix)

	loginBody, _ := json.Marshal(map[string]any{
		"provider":    "r3_test",
		"provider_id": "owner-" + suffix,
	})
	login := performR3Request(
		engine,
		http.MethodPost,
		"/api/v1/auth/login",
		loginBody,
		"",
		"",
	)
	if login.Code != http.StatusOK {
		t.Fatalf("V1 login regression: status=%d body=%s", login.Code, login.Body.String())
	}

	captureID := uuid.New()
	captureBody := r3CaptureBody(captureID, "R3 reliable online capture")
	durations := make([]time.Duration, 0, 100)
	for attempt := 0; attempt < 100; attempt++ {
		started := time.Now()
		recorder := performR3Request(
			engine,
			http.MethodPost,
			"/api/v3/captures",
			captureBody,
			ownerToken,
			captureID.String(),
		)
		durations = append(durations, time.Since(started))
		wantStatus := http.StatusOK
		if attempt == 0 {
			wantStatus = http.StatusCreated
		}
		if recorder.Code != wantStatus {
			t.Fatalf(
				"replay %d: status=%d want=%d body=%s",
				attempt+1,
				recorder.Code,
				wantStatus,
				recorder.Body.String(),
			)
		}
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[94]
	t.Logf("R3_CAPTURE_CREATE_P95_MS=%.3f", float64(p95.Microseconds())/1000)
	if p95 >= 500*time.Millisecond {
		t.Fatalf("capture create P95=%v, target <500ms", p95)
	}

	for table, query := range map[string]string{
		"captures":       "SELECT count(*) FROM captures WHERE user_id = $1 AND id = $2",
		"memory_cards":   "SELECT count(*) FROM memory_cards WHERE user_id = $1 AND capture_id = $2",
		"capture_outbox": "SELECT count(*) FROM capture_outbox WHERE user_id = $1 AND capture_id = $2",
	} {
		var count int
		if err := pool.QueryRow(ctx, query, ownerID, captureID).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 1 {
			t.Fatalf("%s count=%d, want 1", table, count)
		}
	}

	ownerRead := performR3Request(
		engine,
		http.MethodGet,
		"/api/v3/captures/"+captureID.String(),
		nil,
		ownerToken,
		"",
	)
	if ownerRead.Code != http.StatusOK {
		t.Fatalf("owner read: status=%d body=%s", ownerRead.Code, ownerRead.Body.String())
	}
	otherRead := performR3Request(
		engine,
		http.MethodGet,
		"/api/v3/captures/"+captureID.String(),
		nil,
		otherToken,
		"",
	)
	if otherRead.Code != http.StatusNotFound {
		t.Fatalf("cross-user read: status=%d body=%s", otherRead.Code, otherRead.Body.String())
	}

	conflict := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/captures",
		r3CaptureBody(captureID, "different content"),
		ownerToken,
		captureID.String(),
	)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("idempotency conflict: status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	emptyID := uuid.New()
	empty := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/captures",
		r3CaptureBody(emptyID, "   "),
		ownerToken,
		emptyID.String(),
	)
	if empty.Code != http.StatusBadRequest {
		t.Fatalf("empty text: status=%d body=%s", empty.Code, empty.Body.String())
	}

	longID := uuid.New()
	tooLong := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/captures",
		r3CaptureBody(longID, strings.Repeat("想", service.MaxCaptureTextRunes+1)),
		ownerToken,
		longID.String(),
	)
	if tooLong.Code != http.StatusBadRequest {
		t.Fatalf("too long text: status=%d body=%s", tooLong.Code, tooLong.Body.String())
	}

	oversizedID := uuid.New()
	oversized := []byte(fmt.Sprintf(
		"{\"capture_id\":%q,\"kind\":\"text\",\"text\":%q,\"client_version\":1}",
		oversizedID.String(),
		strings.Repeat("x", maxCaptureRequestBodyBytes),
	))
	oversizedResponse := performR3Request(
		engine,
		http.MethodPost,
		"/api/v3/captures",
		oversized,
		ownerToken,
		oversizedID.String(),
	)
	if oversizedResponse.Code != http.StatusBadRequest {
		t.Fatalf(
			"oversized request: status=%d body=%s",
			oversizedResponse.Code,
			oversizedResponse.Body.String(),
		)
	}
}

func r3CaptureBody(captureID uuid.UUID, text string) []byte {
	body, err := json.Marshal(map[string]any{
		"capture_id":            captureID,
		"kind":                  "text",
		"text":                  text,
		"captured_at_precision": "exact",
		"source":                "r3_acceptance",
		"privacy_mode":          "cloud_allowed",
		"client_version":        1,
	})
	if err != nil {
		panic(err)
	}
	return body
}

func performR3Request(
	handler http.Handler,
	method string,
	path string,
	body []byte,
	token string,
	idempotencyKey string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotencyKey != "" {
		request.Header.Set(idempotencyKeyHeader, idempotencyKey)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}
