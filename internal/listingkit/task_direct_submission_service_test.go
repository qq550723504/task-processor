package listingkit

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskDirectSubmissionServiceSubmitSheinTaskDirectStopsOnReadinessFailure(t *testing.T) {
	t.Parallel()

	task := makeReadySheinTask()
	task.Result.Shein.PreviewProduct = nil
	var normalizeCalled bool
	var failCalled bool
	direct := newTaskDirectSubmissionService(taskDirectSubmissionServiceConfig{
		normalizeSheinSubmitPackage: func(task *Task, pkg *SheinPackage, req *SubmitTaskRequest, action string) {
			normalizeCalled = true
		},
		failSheinDirectSubmit: func(_ context.Context, _ string, _ *Task, _ *SheinPackage, _ string, submitErr error) error {
			failCalled = true
			return submitErr
		},
	})

	_, err := direct.submitSheinTaskDirect(context.Background(), "task-1", task, &SubmitTaskRequest{Platform: "shein", Action: "publish"}, sheinDirectSubmitOptions{
		action:    "publish",
		requestID: "req-1",
		startedAt: time.Now(),
	})
	if err == nil || !errors.Is(err, ErrSubmitBlocked) {
		t.Fatalf("submitSheinTaskDirect() err = %v, want ErrSubmitBlocked", err)
	}
	if !normalizeCalled {
		t.Fatal("expected normalizeSheinSubmitPackage to be called")
	}
	if !failCalled {
		t.Fatal("expected failSheinDirectSubmit to be called")
	}
}
