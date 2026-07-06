package listingkit

import "testing"

func TestProcessorStateMachineCanProcess(t *testing.T) {
	t.Parallel()

	sm := NewProcessorStateMachine(2)

	tests := []struct {
		name    string
		task    *Task
		wantErr bool
	}{
		{name: "pending task", task: &Task{Status: TaskStatusPending}},
		{name: "nil task", task: nil, wantErr: true},
		{name: "completed task", task: &Task{Status: TaskStatusCompleted}, wantErr: true},
		{name: "needs review task", task: &Task{Status: TaskStatusNeedsReview}, wantErr: true},
		{name: "failed task", task: &Task{Status: TaskStatusFailed}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.CanProcess(tt.task)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestProcessorStateMachineShouldRetry(t *testing.T) {
	t.Parallel()

	sm := NewProcessorStateMachine(2)

	tests := []struct {
		name string
		task *Task
		want bool
	}{
		{name: "nil task", task: nil, want: false},
		{name: "first retry", task: &Task{Status: TaskStatusPending, RetryCount: 0}, want: true},
		{name: "second retry", task: &Task{Status: TaskStatusPending, RetryCount: 1}, want: true},
		{name: "max retries reached", task: &Task{Status: TaskStatusPending, RetryCount: 2}, want: false},
		{name: "completed task", task: &Task{Status: TaskStatusCompleted, RetryCount: 0}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sm.ShouldRetry(tt.task); got != tt.want {
				t.Fatalf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}
