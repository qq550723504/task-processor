package httpapi

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/listingkit"
)

func TestTaskRecoverySweepLoopCloseWaitsForInFlightSweep(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var calls atomic.Int32
	service := &taskRecoverySweepLoopTestService{
		run: func(ctx context.Context, now time.Time, limit int) (int64, error) {
			calls.Add(1)
			started <- struct{}{}
			<-release
			return 0, nil
		},
	}

	ticks := make(chan time.Time)
	closeLoop := startTaskRecoverySweepLoop(taskRecoverySweepLoopConfig{
		recoveryService: service,
		logger:          logrus.New(),
		limit:           20,
		now: func() time.Time {
			return time.Date(2026, 6, 6, 18, 0, 0, 0, time.UTC)
		},
		ticks: ticks,
	})

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("startup recovery sweep did not begin")
	}

	closed := make(chan error, 1)
	go func() {
		closed <- closeLoop()
	}()

	select {
	case err := <-closed:
		t.Fatalf("close returned early with %v, want wait for in-flight sweep", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("close returned error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not finish after in-flight sweep completed")
	}

	if calls.Load() != 1 {
		t.Fatalf("RunRecoverySweep calls = %d, want 1", calls.Load())
	}
}

type taskRecoverySweepLoopTestService struct {
	run func(ctx context.Context, now time.Time, limit int) (int64, error)
}

func (s *taskRecoverySweepLoopTestService) RecoverTaskNow(context.Context, string) (*listingkit.Task, error) {
	return nil, errors.New("not implemented")
}

func (s *taskRecoverySweepLoopTestService) RunRecoverySweep(ctx context.Context, now time.Time, limit int) (int64, error) {
	if s.run == nil {
		return 0, nil
	}
	return s.run(ctx, now, limit)
}

func (s *taskRecoverySweepLoopTestService) BulkRecoverTasks(context.Context, *listingkit.RecoverBlockedTasksQuery) (int64, error) {
	return 0, errors.New("not implemented")
}
