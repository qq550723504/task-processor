package pipeline_test

import (
	"errors"
	"testing"

	"task-processor/internal/productenrich"
	"task-processor/internal/productenrich/pipeline"
)

func TestTaskStateMachine_CanProcess(t *testing.T) {
	sm := pipeline.NewTaskStateMachine(3)

	cases := []struct {
		name    string
		task    *productenrich.Task
		wantErr bool
	}{
		{name: "pending is processable", task: &productenrich.Task{Status: productenrich.TaskStatusPending}},
		{name: "completed is not processable", task: &productenrich.Task{Status: productenrich.TaskStatusCompleted}, wantErr: true},
		{name: "processing is not processable", task: &productenrich.Task{Status: productenrich.TaskStatusProcessing}, wantErr: true},
		{name: "failed is not processable", task: &productenrich.Task{Status: productenrich.TaskStatusFailed}, wantErr: true},
		{name: "nil task", task: nil, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := sm.CanProcess(tc.task)
			if tc.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTaskStateMachine_ClassifyFailure(t *testing.T) {
	sm := pipeline.NewTaskStateMachine(3)

	if got := sm.ClassifyFailure(errors.New("x")); got != productenrich.FailureDispositionRetryable {
		t.Fatalf("plain error classified as %q, want retryable", got)
	}
	if got := sm.ClassifyFailure(productenrich.NewNoRetryError(errors.New("x"))); got != productenrich.FailureDispositionNoRetry {
		t.Fatalf("errNoRetry classified as %q, want no_retry", got)
	}
}

func TestTaskStateMachine_ShouldRetry(t *testing.T) {
	sm := pipeline.NewTaskStateMachine(3)

	if !sm.ShouldRetry(&productenrich.Task{RetryCount: 1}) {
		t.Fatal("expected retry_count=1 to still retry")
	}
	if sm.ShouldRetry(&productenrich.Task{RetryCount: 3}) {
		t.Fatal("expected retry_count=3 to stop retrying")
	}
}
