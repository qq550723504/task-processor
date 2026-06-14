package submission

import (
	"sync"
	"time"
)

const submitLockCleanupThreshold = 10 * time.Minute

// SubmitLockManager manages in-process submission locks by resource key.
type SubmitLockManager struct {
	locks sync.Map // key: string, value: *submitLockEntry
}

type submitLockEntry struct {
	mu       sync.Mutex
	lastUsed time.Time
}

// NewSubmitLockManager creates an in-process submit lock manager.
func NewSubmitLockManager() *SubmitLockManager {
	return &SubmitLockManager{}
}

// Lock acquires the lock for key and returns an unlock function.
func (m *SubmitLockManager) Lock(key string) func() {
	if m == nil {
		return func() {}
	}

	actual, _ := m.locks.LoadOrStore(key, &submitLockEntry{lastUsed: time.Now()})
	entry := actual.(*submitLockEntry)

	entry.lastUsed = time.Now()
	entry.mu.Lock()

	return func() {
		entry.mu.Unlock()
		m.maybeCleanup(key, entry)
	}
}

func (m *SubmitLockManager) maybeCleanup(key string, entry *submitLockEntry) {
	if time.Since(entry.lastUsed) > submitLockCleanupThreshold {
		if time.Since(entry.lastUsed) > submitLockCleanupThreshold {
			m.locks.Delete(key)
		}
	}
}

// Cleanup removes stale locks.
func (m *SubmitLockManager) Cleanup() {
	if m == nil {
		return
	}
	now := time.Now()
	m.locks.Range(func(key, value any) bool {
		entry := value.(*submitLockEntry)
		if now.Sub(entry.lastUsed) > submitLockCleanupThreshold {
			m.locks.Delete(key)
		}
		return true
	})
}
