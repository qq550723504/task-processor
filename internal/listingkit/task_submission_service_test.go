package listingkit

import (
	"context"
	"testing"
	"time"
)

func TestTaskSubmissionServiceSubmitTaskRoutesSheinPublishToWorkflow(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	var workflowCalls int
	var directCalls int
	submitter := newTaskSubmissionService(taskSubmissionServiceConfig{
		lockSubmit: func(string) func() { return func() {} },
		acquireSheinSubmitTask: func(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
			return task, nil, nil
		},
		shouldStartSheinPublishWorkflow: func(platform, action string) bool {
			return platform == "shein" && action == "publish"
		},
		submitSheinTaskWithWorkflow: func(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinWorkflowSubmitOptions) (*ListingKitPreview, error) {
			workflowCalls++
			return &ListingKitPreview{TaskID: taskID}, nil
		},
		submitSheinTaskDirect: func(ctx context.Context, taskID string, task *Task, req *SubmitTaskRequest, opts sheinDirectSubmitOptions) (*ListingKitPreview, error) {
			directCalls++
			return nil, nil
		},
	})

	preview, err := submitter.SubmitTask(context.Background(), task.ID, &SubmitTaskRequest{
		Platform:       "shein",
		Action:         "publish",
		IdempotencyKey: "temporal-route-123",
	})
	if err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}
	if preview == nil || preview.TaskID != task.ID {
		t.Fatalf("preview = %+v, want preview for task", preview)
	}
	if workflowCalls != 1 {
		t.Fatalf("workflow calls = %d, want 1", workflowCalls)
	}
	if directCalls != 0 {
		t.Fatalf("direct calls = %d, want 0", directCalls)
	}
}
