package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
)

// DefaultRetryPolicy returns the standard retry policy for activities.
func DefaultRetryPolicy() *temporal.RetryPolicy {
	return &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		MaximumInterval:    time.Second * 10,
		BackoffCoefficient: 2,
		MaximumAttempts:    3,
	}
}

// DefaultActivityOptions returns standard activity options with timeout and retry.
func DefaultActivityOptions() *WorkflowActivityOptions {
	return &WorkflowActivityOptions{
		StartToCloseTimeout: time.Second * 30,
		RetryPolicy:         DefaultRetryPolicy(),
	}
}

// WorkflowActivityOptions holds common activity options for workflows.
type WorkflowActivityOptions struct {
	StartToCloseTimeout time.Duration
	RetryPolicy         *temporal.RetryPolicy
}
