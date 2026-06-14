package submission

import (
	"sync"
	"testing"
	"time"
)

func TestSubmitLockManagerBasicLock(t *testing.T) {
	manager := NewSubmitLockManager()

	unlock := manager.Lock("task-123")

	done := make(chan bool)
	go func() {
		defer func() { done <- true }()
		secondUnlock := manager.Lock("task-123")
		secondUnlock()
	}()

	select {
	case <-done:
		t.Fatal("lock should block until first unlock")
	case <-time.After(50 * time.Millisecond):
	}

	unlock()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("second lock should acquire after first unlock")
	}
}

func TestSubmitLockManagerDifferentKeys(t *testing.T) {
	manager := NewSubmitLockManager()

	unlock1 := manager.Lock("task-1")
	unlock2 := manager.Lock("task-2")

	if unlock1 == nil || unlock2 == nil {
		t.Fatal("different keys should both acquire locks")
	}

	unlock1()
	unlock2()
}

func TestSubmitLockManagerConcurrentAccess(t *testing.T) {
	manager := NewSubmitLockManager()

	var wg sync.WaitGroup
	counter := 0

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := manager.Lock("shared-resource")
			counter++
			time.Sleep(time.Millisecond)
			unlock()
		}()
	}

	wg.Wait()

	if counter != 100 {
		t.Fatalf("counter = %d, want 100", counter)
	}
}

func TestSubmitLockManagerCleanupAndNilManager(t *testing.T) {
	manager := NewSubmitLockManager()

	manager.Lock("key-1")()
	manager.Lock("key-2")()
	manager.Cleanup()
	manager.Lock("key-3")()

	var nilManager *SubmitLockManager
	nilManager.Lock("test-key")()
	nilManager.Cleanup()
}
