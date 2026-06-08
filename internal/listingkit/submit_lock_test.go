package listingkit

import (
	"sync"
	"testing"
	"time"
)

func TestSubmitLockManager_BasicLock(t *testing.T) {
	mgr := newSubmitLockManager()

	unlocked := false
	unlock := mgr.lock("task-123")

	// 验证锁已获取
	done := make(chan bool)
	go func() {
		mgr.lock("task-123") // 这会被阻塞
		done <- true
	}()

	// 等待一小段时间,确认 goroutine 被阻塞
	time.Sleep(100 * time.Millisecond)

	// 解锁
	unlock()
	unlocked = true

	// 验证 goroutine 现在可以继续
	select {
	case <-done:
		// 成功
	case <-time.After(1 * time.Second):
		t.Fatal("lock was not released")
	}

	if !unlocked {
		t.Error("unlock function was not called")
	}
}

func TestSubmitLockManager_DifferentKeys(t *testing.T) {
	mgr := newSubmitLockManager()

	// 不同 key 的锁不应该互相阻塞
	unlock1 := mgr.lock("task-1")
	unlock2 := mgr.lock("task-2")

	// 两个锁都应该能立即获取
	unlock1()
	unlock2()
}

func TestSubmitLockManager_ConcurrentAccess(t *testing.T) {
	mgr := newSubmitLockManager()

	var wg sync.WaitGroup
	counter := 0

	// 100 个 goroutine 竞争同一个锁
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := mgr.lock("shared-resource")
			counter++
			time.Sleep(1 * time.Millisecond) // 模拟工作
			unlock()
		}()
	}

	wg.Wait()

	if counter != 100 {
		t.Errorf("counter = %d, want 100", counter)
	}
}

func TestSubmitLockManager_Cleanup(t *testing.T) {
	mgr := newSubmitLockManager()

	// 创建一些锁
	mgr.lock("key-1")()
	mgr.lock("key-2")()
	mgr.lock("key-3")()

	// 触发清理
	mgr.Cleanup()

	// 验证清理后仍能正常工作
	unlock := mgr.lock("key-4")
	unlock()
}

func TestSubmitLockManager_NilManager(t *testing.T) {
	var mgr *submitLockManager

	// 应该返回一个空的 unlock 函数,不会 panic
	unlock := mgr.lock("test-key")
	unlock() // 应该不会 panic
}
