package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// registerWorkflowGuardRoutes reserves the /api/v3/workflows namespace for
// later rounds. Any request that reaches a workflow route in the MVP build
// returns 501 FEATURE_NOT_ENABLED — the server never creates a workflow run,
// a background task, or any external side effect (see G8 completion criteria
// and DOMAIN_AND_AI_SPEC_V3 §4.5).
//
// The 501 status is deliberate: it signals "this server build does not offer
// the feature". Per-user AI toggles that a user switched off keep returning
// 409 FEATURE_NOT_ENABLED (R8/R9), which is a different failure: the server
// offers the feature but the account has it disabled.
func registerWorkflowGuardRoutes(group *gin.RouterGroup) {
	group.Any("/workflows", workflowNotEnabled)
	group.Any("/workflows/*any", workflowNotEnabled)
}

func workflowNotEnabled(c *gin.Context) {
	writeV3Error(
		c,
		http.StatusNotImplemented,
		V3ErrorFeatureNotEnabled,
		"workflows are not available in this build",
		map[string]any{
			"workflow_designer":  false,
			"workflow_execution": false,
		},
	)
}
