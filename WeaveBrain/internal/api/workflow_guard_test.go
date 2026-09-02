package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func newWorkflowGuardTestEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	registerWorkflowGuardRoutes(engine.Group(apiV3Prefix))
	return engine
}

// Every method/path under /api/v3/workflows must return 501 FEATURE_NOT_ENABLED
// in the MVP build — the server never creates a run, a background task, or an
// external side effect (G8 completion criterion).
func TestWorkflowGuardReturnsFeatureNotEnabled(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "create workflow", method: http.MethodPost, path: "/api/v3/workflows"},
		{name: "list workflows", method: http.MethodGet, path: "/api/v3/workflows"},
		{name: "get workflow", method: http.MethodGet, path: "/api/v3/workflows/wf-1"},
		{name: "draft", method: http.MethodPut, path: "/api/v3/workflows/wf-1/draft"},
		{name: "validate", method: http.MethodPost, path: "/api/v3/workflows/wf-1/validate"},
		{name: "publish", method: http.MethodPost, path: "/api/v3/workflows/wf-1/publish"},
		{name: "run", method: http.MethodPost, path: "/api/v3/workflows/wf-1/run"},
		{name: "delete", method: http.MethodDelete, path: "/api/v3/workflows/wf-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newWorkflowGuardTestEngine()
			requestID := uuid.NewString()
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set(requestIDHeader, requestID)
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, req)

			if recorder.Code != http.StatusNotImplemented {
				t.Fatalf("expected 501, got %d: %s", recorder.Code, recorder.Body.String())
			}
			assertV3ErrorResponse(
				t,
				recorder,
				http.StatusNotImplemented,
				V3ErrorFeatureNotEnabled,
				requestID,
			)

			var response V3ErrorResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if got, ok := response.Details["workflow_designer"].(bool); !ok || got {
				t.Fatalf("expected details.workflow_designer=false, got %v", response.Details["workflow_designer"])
			}
			if got, ok := response.Details["workflow_execution"].(bool); !ok || got {
				t.Fatalf("expected details.workflow_execution=false, got %v", response.Details["workflow_execution"])
			}
		})
	}
}

// The guard is registered under the protected prefix in the real server, so a
// request without a bearer token must be rejected by auth (401) before the
// guard can fire. This test proves the two route layers coexist.
func TestWorkflowGuardDoesNotShadowProtectedPrefixAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(requestIDMiddleware())
	group := engine.Group(apiV3Prefix)
	registerWorkflowGuardRoutes(group)
	// A sibling protected handler on a different path must still register.
	group.Any("/captures", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v3/workflows", nil)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("expected guard 501, got %d", recorder.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v3/captures", nil)
	recorder = httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected sibling route 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
