package listingkit

import "context"

// RetryChildTaskRequest retriggers a specific child task kind.
type RetryChildTaskRequest struct {
	Kind    string         `json:"kind"`
	Options map[string]any `json:"options,omitempty"`
}

type ChildTaskRetryService interface {
	RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error)
}
