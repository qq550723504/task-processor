package workerpool

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

type gatedProcessor struct {
	firstStarted chan struct{}
	releaseFirst chan struct{}
	processed    chan int64
}

func newGatedProcessor() *gatedProcessor {
	return &gatedProcessor{
		firstStarted: make(chan struct{}),
		releaseFirst: make(chan struct{}),
		processed:    make(chan int64, 2),
	}
}

func (*gatedProcessor) Start(context.Context) error { return nil }

func (p *gatedProcessor) ProcessTask(ctx context.Context, job WorkerJob) error {
	if job.TaskID == 1 {
		close(p.firstStarted)
		select {
		case <-p.releaseFirst:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	p.processed <- job.TaskID
	return nil
}

func (*gatedProcessor) Close(context.Context) {}

func TestPoolStopWaitsForActiveTaskAndDrainsQueuedTasks(t *testing.T) {
	processor := newGatedProcessor()
	pool := NewPoolWithConfig(processor, PoolConfig{
		Concurrency:     1,
		BufferSize:      2,
		TaskTimeout:     time.Minute,
		ShutdownTimeout: time.Minute,
	})
	pool.Start(context.Background())

	if err := pool.Submit(WorkerJob{TaskID: 1}); err != nil {
		t.Fatalf("Submit(first) error = %v", err)
	}
	select {
	case <-processor.firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first task did not start")
	}
	if err := pool.Submit(WorkerJob{TaskID: 2}); err != nil {
		t.Fatalf("Submit(second) error = %v", err)
	}

	stopDone := make(chan struct{})
	go func() {
		pool.Stop(context.Background())
		close(stopDone)
	}()
	waitForPoolClosed(t, pool)
	select {
	case <-stopDone:
		t.Fatal("Stop returned before the active task was released")
	default:
	}

	close(processor.releaseFirst)
	processed := map[int64]bool{}
	for len(processed) < 2 {
		select {
		case taskID := <-processor.processed:
			processed[taskID] = true
		case <-time.After(time.Second):
			t.Fatalf("processed tasks = %#v, want task IDs 1 and 2", processed)
		}
	}
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after queued tasks drained")
	}
	if !processed[1] || !processed[2] {
		t.Fatalf("processed tasks = %#v, want task IDs 1 and 2", processed)
	}
}

func TestPoolSubmitAfterStopReturnsErrPoolClosed(t *testing.T) {
	pool := NewPoolWithConfig(newGatedProcessor(), PoolConfig{
		Concurrency:     1,
		BufferSize:      1,
		TaskTimeout:     time.Minute,
		ShutdownTimeout: time.Minute,
	})
	pool.Stop(context.Background())

	if err := pool.Submit(WorkerJob{TaskID: 1}); !errors.Is(err, ErrPoolClosed) {
		t.Fatalf("Submit() error = %v, want ErrPoolClosed", err)
	}
}

func waitForPoolClosed(t *testing.T, pool *Pool) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		pool.mu.RLock()
		closed := pool.closed
		pool.mu.RUnlock()
		if closed {
			return
		}
		select {
		case <-deadline:
			t.Fatal("Stop did not close the pool")
		default:
			runtime.Gosched()
		}
	}
}
