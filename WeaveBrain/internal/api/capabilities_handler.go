package api

import (
	"net/http"

	"weavebrain/internal/entity"

	"github.com/gin-gonic/gin"
)

// V3CapabilitiesResponse advertises which MVP features the running server
// build offers. Clients use it to gate UI affordances (e.g. hide the workflow
// designer/run entry) and to decide which capture sources a device can push.
type V3CapabilitiesResponse struct {
	MobileCapture           bool     `json:"mobile_capture"`
	WebReview               bool     `json:"web_review"`
	WorkflowDesigner        bool     `json:"workflow_designer"`
	WorkflowExecution       bool     `json:"workflow_execution"`
	SupportedCaptureSources []string `json:"supported_capture_sources"`
}

// v3Capabilities is the static capability set of the current MVP build.
// Workflow design/execution is reserved for later rounds; the /workflows
// namespace returns 501 FEATURE_NOT_ENABLED (see workflow_guard.go).
func v3Capabilities() V3CapabilitiesResponse {
	return V3CapabilitiesResponse{
		MobileCapture:     true,
		WebReview:         true,
		WorkflowDesigner:  false,
		WorkflowExecution: false,
		SupportedCaptureSources: []string{
			string(entity.CaptureKindText),
			string(entity.CaptureKindAudio),
			string(entity.CaptureKindImport),
		},
	}
}

func capabilitiesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, v3Capabilities())
}
