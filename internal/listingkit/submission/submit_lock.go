package submission

import (
	"sync"
	"time"
)

// SubmitLockManager 管理提交锁,防止同一资源并发提交
type SubmitLockManager struct {
	locks sync.Map // key: string, value: *entry
}

type entry struct {
	mu       sync.Mutex
	lastUsed time.Time
}

// cleanupThreshold 锁多久未使用后可以被清理
const cleanupThreshold = 10 * time.Minute

// NewSubmitLockManager 创建新的提交锁管理器
func NewSubmitLockManager() *SubmitLockManager {
	return &SubmitLockManager{}
}

// lock 获取指定 key 的锁,返回解锁函数
// Lock 获取锁,返回解锁函数
func (m *SubmitLockManager) Lock(key string) func() {
	if m == nil {
		return func() {}
	}

	// 获取或创建 entry
	actual, _ := m.locks.LoadOrStore(key, &entry{lastUsed: time.Now()})
	e := actual.(*entry)

	// 更新最后使用时间
	e.lastUsed = time.Now()

	// 获取锁
	e.mu.Lock()

	// 返回解锁函数,解锁时标记为可清理
	return func() {
		e.mu.Unlock()
		// 惰性清理: 解锁后检查是否可以删除
		m.maybeCleanup(key, e)
	}
}

// maybeCleanup 尝试清理长时间未使用的锁
func (m *SubmitLockManager) maybeCleanup(key string, e *entry) {
	// 如果超过阈值未使用,尝试删除
	if time.Since(e.lastUsed) > cleanupThreshold {
		// 双重检查: 再次确认 lastUsed
		if time.Since(e.lastUsed) > cleanupThreshold {
			m.locks.Delete(key)
		}
	}
}

// Cleanup 主动清理所有过期的锁(可选,可以定期调用)
// Cleanup 主动清理未使用的锁
func (m *SubmitLockManager) Cleanup() {
	now := time.Now()
	m.locks.Range(func(key, value interface{}) bool {
		e := value.(*entry)
		if now.Sub(e.lastUsed) > cleanupThreshold {
			m.locks.Delete(key)
		}
		return true
	})
}
