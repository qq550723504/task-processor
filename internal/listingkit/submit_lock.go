package listingkit

import "sync"

type submitLockManager struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newSubmitLockManager() *submitLockManager {
	return &submitLockManager{locks: map[string]*sync.Mutex{}}
}

func (m *submitLockManager) lock(key string) func() {
	if m == nil {
		return func() {}
	}
	m.mu.Lock()
	lock := m.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		m.locks[key] = lock
	}
	m.mu.Unlock()

	lock.Lock()
	return lock.Unlock
}
