package submission

import (
	"sync"
	"testing"
	"time"
)

func TestSubmitLockManager_BasicLock(t *testing.T) {
	mgr := NewSubmitLockManager()

	unlocked := false
	unlock := mgr.Lock("task-123")

	// 验证锁已获取
	done := make(chan bool)
	go func() {
		mgr.Lock("task-123") // 这会被阻塞
		done <- true
	}()

	select {
	case <-done:
		t.Error("锁应该被阻塞,但立即返回了")
	case <-time.After(50 * time.Millisecond):
		// 预期行为: 超时,说明锁正常工作
	}

	unlock()
	unlocked = true

	// 现在第二个 goroutine 应该能获取锁
	select {
	case <-done:
		// 预期行为
	case <-time.After(100 * time.Millisecond):
		t.Error("解锁后,第二个 goroutine 应该能获取锁")
	}

	if !unlocked {
		t.Error("unlock 函数应该被调用")
	}
}

func TestSubmitLockManager_DifferentKeys(t *testing.T) {
	mgr := NewSubmitLockManager()

	// 不同 key 的锁不应该互相阻塞
	unlock1 := mgr.Lock("task-1")
	unlock2 := mgr.Lock("task-2")

	// 两个锁都应该能立即获取
	if unlock1 == nil || unlock2 == nil {
		t.Error("不同 key 的锁应该都能获取")
	}

	unlock1()
	unlock2()
}

func TestSubmitLockManager_ConcurrentAccess(t *testing.T) {
	mgr := NewSubmitLockManager()

	var wg sync.WaitGroup
	counter := 0

	// 启动多个 goroutine 竞争同一个锁
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := mgr.Lock("shared-resource")
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
	mgr := NewSubmitLockManager()

	// 创建一些锁
	mgr.Lock("key-1")()
	mgr.Lock("key-2")()
	mgr.Lock("key-3")()

	// 触发清理
	mgr.Cleanup()

	// 验证清理后仍能正常工作
	unlock := mgr.Lock("key-4")
	unlock()
}

func TestSubmitLockManager_NilManager(t *testing.T) {
	var mgr *SubmitLockManager

	// 应该返回一个空的 unlock 函数,不会 panic
	unlock := mgr.Lock("test-key")
	unlock() // 应该不会 panic
}
