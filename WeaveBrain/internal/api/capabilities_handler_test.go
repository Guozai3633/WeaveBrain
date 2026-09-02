package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"weavebrain/internal/entity"
)

func TestV3CapabilitiesEndpoint(t *testing.T) {
	engine := newV3ContractTestEngine()
	req := httptest.NewRequest(http.MethodGet, "/api/v3/capabilities", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}

	var response V3CapabilitiesResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.MobileCapture {
		t.Fatal("expected mobile_capture=true in MVP build")
	}
	if !response.WebReview {
		t.Fatal("expected web_review=true in MVP build")
	}
	if response.WorkflowDesigner {
		t.Fatal("expected workflow_designer=false (reserved for later rounds)")
	}
	if response.WorkflowExecution {
		t.Fatal("expected workflow_execution=false (reserved for later rounds)")
	}

	wantSources := []string{
		string(entity.CaptureKindText),
		string(entity.CaptureKindAudio),
		string(entity.CaptureKindImport),
	}
	if len(response.SupportedCaptureSources) != len(wantSources) {
		t.Fatalf(
			"expected %d supported capture sources, got %d: %v",
			len(wantSources),
			len(response.SupportedCaptureSources),
			response.SupportedCaptureSources,
		)
	}
	for i, want := range wantSources {
		if response.SupportedCaptureSources[i] != want {
			t.Fatalf(
				"supported_capture_sources[%d]=%q, want %q",
				i,
				response.SupportedCaptureSources[i],
				want,
			)
		}
	}
}

// Capabilities is public: it must be reachable without a bearer token, unlike
// the protected domain routes that share the /api/v3 prefix.
func TestV3CapabilitiesIsPublic(t *testing.T) {
	engine := newV3ContractTestEngine()
	req := httptest.NewRequest(http.MethodGet, "/api/v3/capabilities", nil)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected public 200 without auth, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
