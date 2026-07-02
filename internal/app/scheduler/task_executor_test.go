package scheduler

import (
	"context"
	"testing"
	"time"

	infralock "task-processor/internal/infra/lock"
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

type countingTask struct {
	id       string
	taskType TaskType
	platform string
	storeID  int64
	interval time.Duration
	calls    int
}

func (t *countingTask) GetID() string {
	if t.id != "" {
		return t.id
	}
	return "counting-task"
}

func (t *countingTask) GetType() TaskType { return t.taskType }

func (t *countingTask) GetPlatform() string { return t.platform }

func (t *countingTask) GetStoreID() int64 { return t.storeID }

func (t *countingTask) Execute(context.Context) error {
	t.calls++
	return nil
}

func (t *countingTask) GetInterval() time.Duration { return t.interval }

func (t *countingTask) GetStatus() TaskStatus { return TaskStatusRunning }

type recordingLock struct {
	locked     bool
	tryKeys    []string
	unlockKeys []string
}

func (l *recordingLock) TryLock(_ context.Context, key string, _ time.Duration) (bool, error) {
	l.tryKeys = append(l.tryKeys, key)
	if l.locked {
		return false, nil
	}
	l.locked = true
	return true, nil
}

func (l *recordingLock) Unlock(_ context.Context, key string) error {
	l.unlockKeys = append(l.unlockKeys, key)
	l.locked = false
	return nil
}

func (l *recordingLock) Extend(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}

func (l *recordingLock) IsLocked(context.Context, string) (bool, error) {
	return l.locked, nil
}

var _ infralock.DistributedLock = (*recordingLock)(nil)

func TestTaskExecutorStartIsIdempotent(t *testing.T) {
	task := &blockingTask{}
	executor := NewTaskExecutor(context.Background(), task, nil, time.Minute)
	defer executor.Stop()

	if !executor.Start() {
		t.Fatal("expected first start to launch executor")
	}
	if executor.Start() {
		t.Fatal("expected second start to be ignored")
	}
}

func TestTaskExecutorDistributedLockUsesStoreScopedKey(t *testing.T) {
	task := &countingTask{
		id:       "shein:inventory:962",
		taskType: TaskTypeInventory,
		platform: "SHEIN",
		storeID:  962,
		interval: time.Minute,
	}
	locker := &recordingLock{}
	executor := NewTaskExecutor(context.Background(), task, nil, time.Minute)
	executor.SetDistributedLock(locker, 30*time.Second)

	executor.executeTaskWithConcurrencyControl()

	const expectedKey = "listing:scheduler:lock:SHEIN:inventory:962"
	if task.calls != 1 {
		t.Fatalf("expected task to execute once after acquiring lock, got %d", task.calls)
	}
	if len(locker.tryKeys) != 1 || locker.tryKeys[0] != expectedKey {
		t.Fatalf("expected lock key %q, got %v", expectedKey, locker.tryKeys)
	}
	if len(locker.unlockKeys) != 1 || locker.unlockKeys[0] != expectedKey {
		t.Fatalf("expected unlock key %q, got %v", expectedKey, locker.unlockKeys)
	}

	locker.locked = true
	executor.executeTaskWithConcurrencyControl()
	if task.calls != 1 {
		t.Fatalf("expected task to skip when distributed lock is held, calls=%d", task.calls)
	}
}
