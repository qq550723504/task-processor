package sheinsync

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCostRefreshCoordinatorReturnsBeforeRefreshCompletes(t *testing.T) {
	t.Parallel()

	coordinator := newCostRefreshCoordinator(func(string, error) {})
	release := make(chan struct{})
	started := make(chan struct{})
	returned := make(chan struct{})

	go func() {
		coordinator.Schedule("227:870:source:XB0608035002", context.Background(), func(context.Context) error {
			close(started)
			<-release
			return nil
		})
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("scheduling a candidate refresh blocked on the refresh work")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("scheduled candidate refresh did not start")
	}
	close(release)
}

func TestCostRefreshCoordinatorCoalescesConcurrentRequestsIntoOneFollowUpRun(t *testing.T) {
	t.Parallel()

	coordinator := newCostRefreshCoordinator(func(string, error) {})
	releaseFirstRun := make(chan struct{})
	firstRunStarted := make(chan struct{})
	var runs atomic.Int32
	refresh := func(context.Context) error {
		if runs.Add(1) == 1 {
			close(firstRunStarted)
			<-releaseFirstRun
		}
		return nil
	}

	coordinator.Schedule("227:870:source:XB0608035002", context.Background(), refresh)
	require.Eventually(t, func() bool {
		select {
		case <-firstRunStarted:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	coordinator.Schedule("227:870:source:XB0608035002", context.Background(), refresh)
	coordinator.Schedule("227:870:source:XB0608035002", context.Background(), refresh)
	close(releaseFirstRun)

	require.Eventually(t, func() bool {
		return runs.Load() == 2
	}, time.Second, 10*time.Millisecond)

	time.Sleep(50 * time.Millisecond)
	require.EqualValues(t, 2, runs.Load())
}
