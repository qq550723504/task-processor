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
	var maxSeen int32
	var current int32

	go func() {
		_ = runtime.withStoreLock(1, func() error {
			if !runtime.IsInFlight(1) {
				t.Errorf("expected store to be marked in flight")
			}
			atomic.StoreInt32(&current, 1)
			atomic.CompareAndSwapInt32(&maxSeen, 0, 1)
			close(started)
			<-release
			atomic.StoreInt32(&current, 0)
			return nil
		})
	}()

	<-started
	done := make(chan struct{})
	go func() {
		_ = runtime.withStoreLock(1, func() error {
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
	if runtime.IsInFlight(1) {
		t.Fatal("expected in-flight marker to clear")
	}
	if got := runtime.MaxConcurrent(); got != 1 {
		t.Fatalf("unexpected max concurrent: %d", got)
	}
}
