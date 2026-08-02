package sheinlogin

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeWithStoreLockMarksInFlightAndSerializes(t *testing.T) {
	runtime := NewRuntime(1)
	started := make(chan struct{})
	release := make(chan struct{})
	var current int32

	go func() {
		_ = runtime.withStoreLock(1, 1, func() error {
			if !runtime.IsInFlight(1, 1) {
				t.Errorf("expected store to be marked in flight")
			}
			atomic.StoreInt32(&current, 1)
			close(started)
			<-release
			atomic.StoreInt32(&current, 0)
			return nil
		})
	}()

	<-started
	done := make(chan struct{})
	go func() {
		_ = runtime.withStoreLock(1, 1, func() error {
			if got := atomic.LoadInt32(&current); got != 0 {
				t.Errorf("expected serialized execution, got current=%d", got)
			}
			close(done)
			return nil
		})
	}()

	select {
	case <-done:
		t.Fatal("second execution should block until first is released")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second execution did not complete")
	}
	if runtime.IsInFlight(1, 1) {
		t.Fatal("expected in-flight marker to clear")
	}
	if got := runtime.MaxConcurrent(); got != 1 {
		t.Fatalf("unexpected max concurrent: %d", got)
	}
}

func TestRuntimeSeparatesSameStoreAcrossTenants(t *testing.T) {
	runtime := NewRuntime(2)
	started := make(chan int64, 2)
	release := make(chan struct{})

	for _, tenantID := range []int64{1, 9} {
		go func(tenantID int64) {
			_ = runtime.withStoreLock(tenantID, 1, func() error {
				if !runtime.IsInFlight(tenantID, 1) {
					t.Errorf("expected tenant %d store 1 to be marked in flight", tenantID)
				}
				started <- tenantID
				<-release
				return nil
			})
		}(tenantID)
	}

	seen := map[int64]bool{}
	for len(seen) < 2 {
		select {
		case tenantID := <-started:
			seen[tenantID] = true
		case <-time.After(150 * time.Millisecond):
			t.Fatalf("expected both tenant locks to run independently, saw tenants=%v", seen)
		}
	}

	if !runtime.IsInFlight(1, 1) {
		t.Fatal("expected tenant 1 store 1 to still be in flight")
	}
	if !runtime.IsInFlight(9, 1) {
		t.Fatal("expected tenant 9 store 1 to still be in flight")
	}

	close(release)

	deadline := time.After(time.Second)
	for runtime.IsInFlight(1, 1) || runtime.IsInFlight(9, 1) {
		select {
		case <-deadline:
			t.Fatal("expected in-flight markers to clear after both tenants finished")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
