package scheduler

import (
	"context"
	"testing"
	"time"
)

type blockingTask struct {
	deadline    time.Time
	hasDeadline bool
	err         error
}

func (t *blockingTask) GetID() string {
	return "blocking-task"
}

func (t *blockingTask) GetType() TaskType {
	return TaskTypePricing
}

func (t *blockingTask) GetPlatform() string {
	return "test"
}

func (t *blockingTask) GetStoreID() int64 {
	return 1
}

func (t *blockingTask) Execute(ctx context.Context) error {
	t.deadline, t.hasDeadline = ctx.Deadline()
	<-ctx.Done()
	t.err = ctx.Err()
	return t.err
}

func (t *blockingTask) GetInterval() time.Duration {
	return time.Minute
}

func (t *blockingTask) GetStatus() TaskStatus {
	return TaskStatusRunning
}

func TestTaskExecutorUsesConfiguredTimeout(t *testing.T) {
	task := &blockingTask{}
	executor := NewTaskExecutor(context.Background(), task, nil, 50*time.Millisecond)

	start := time.Now()
	executor.executeTask()

	if !task.hasDeadline {
		t.Fatal("expected task context to have a deadline")
	}
	if task.err != context.DeadlineExceeded {
		t.Fatalf("expected deadline exceeded, got %v", task.err)
	}

	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("expected configured timeout to end quickly, elapsed=%v", elapsed)
	}

	remaining := time.Until(task.deadline)
	if remaining < -500*time.Millisecond || remaining > 100*time.Millisecond {
		t.Fatalf("unexpected deadline captured relative to now: %v", remaining)
	}
}
