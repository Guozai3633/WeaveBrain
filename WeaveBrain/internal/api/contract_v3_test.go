package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newV3ContractTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	registerV3ContractRoutes(engine)
	return engine
}

func TestV3MetaContract(t *testing.T) {
	engine := newV3ContractTestEngine()
	requestID := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v3/meta", nil)
	req.Header.Set(requestIDHeader, requestID)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get(requestIDHeader); got != requestID {
		t.Fatalf("expected request id header %q, got %q", requestID, got)
	}

	var response V3MetaResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.APIVersion != "v3" {
		t.Fatalf("expected api version v3, got %q", response.APIVersion)
	}
	if response.RequestID != requestID {
		t.Fatalf("expected request id %q, got %q", requestID, response.RequestID)
	}
}

func TestV3UnknownRouteUsesErrorContract(t *testing.T) {
	engine := newV3ContractTestEngine()
	requestID := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/api/v3/unknown", nil)
	req.Header.Set(requestIDHeader, requestID)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	assertV3ErrorResponse(
		t,
		recorder,
		http.StatusNotFound,
		V3ErrorNotFound,
		requestID,
	)
}

func TestV3ErrorStatusContracts(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   V3ErrorCode
	}{
		{name: "bad request", status: http.StatusBadRequest, code: V3ErrorInvalidArgument},
		{name: "unauthorized", status: http.StatusUnauthorized, code: V3ErrorUnauthorized},
		{name: "forbidden", status: http.StatusForbidden, code: V3ErrorForbidden},
		{name: "not found", status: http.StatusNotFound, code: V3ErrorNotFound},
		{name: "idempotency conflict", status: http.StatusConflict, code: V3ErrorIdempotencyConflict},
		{name: "version conflict", status: http.StatusConflict, code: V3ErrorVersionConflict},
		{name: "feature disabled", status: http.StatusNotImplemented, code: V3ErrorFeatureNotEnabled},
		{name: "internal", status: http.StatusInternalServerError, code: V3ErrorInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(requestIDMiddleware())
			engine.GET("/test", func(c *gin.Context) {
				writeV3Error(c, tt.status, tt.code, "test message", nil)
			})

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, req)

			requestID := recorder.Header().Get(requestIDHeader)
			if _, err := uuid.Parse(requestID); err != nil {
				t.Fatalf("expected generated UUID request id, got %q", requestID)
			}
			assertV3ErrorResponse(t, recorder, tt.status, tt.code, requestID)
		})
	}
}

func TestRequestIDMiddlewareReplacesInvalidValue(t *testing.T) {
	engine := newV3ContractTestEngine()
	req := httptest.NewRequest(http.MethodGet, "/api/v3/meta", nil)
	req.Header.Set(requestIDHeader, "not-a-uuid")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	requestID := recorder.Header().Get(requestIDHeader)
	if requestID == "not-a-uuid" {
		t.Fatal("expected invalid request id to be replaced")
	}
	if _, err := uuid.Parse(requestID); err != nil {
		t.Fatalf("expected generated UUID request id, got %q", requestID)
	}
}

func assertV3ErrorResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	expectedStatus int,
	expectedCode V3ErrorCode,
	expectedRequestID string,
) {
	t.Helper()

	if recorder.Code != expectedStatus {
		t.Fatalf("expected status %d, got %d", expectedStatus, recorder.Code)
		if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
			t.Fatalf("expected JSON content type, got %q", got)
		}
	}

	var response V3ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Code != expectedCode {
		t.Fatalf("expected code %q, got %q", expectedCode, response.Code)
	}
	if response.Message == "" {
		t.Fatal("expected non-empty error message")
	}
	if response.RequestID != expectedRequestID {
		t.Fatalf(
			"expected request id %q, got %q",
			expectedRequestID,
			response.RequestID,
		)
	}
	if response.Details == nil {
		t.Fatal("expected details to be an object, got nil")
	}
}

func TestLegacyUnknownRouteKeepsLegacyResponseShape(t *testing.T) {
	engine := newV3ContractTestEngine()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
	if recorder.Body.String() != "404 page not found" {
		t.Fatalf("unexpected legacy response body %q", recorder.Body.String())
	}
}
