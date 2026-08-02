package sheinlogin

import (
	"sync"
)

type Runtime struct {
	mu            sync.Mutex
	locks         map[runtimeStoreKey]*sync.Mutex
	inFlight      map[runtimeStoreKey]bool
	semaphore     chan struct{}
	maxConcurrent int
}

type runtimeStoreKey struct {
	tenantID int64
	storeID  int64
}

func NewRuntime(maxConcurrent int) *Runtime {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &Runtime{
		locks:         make(map[runtimeStoreKey]*sync.Mutex),
		inFlight:      make(map[runtimeStoreKey]bool),
		semaphore:     make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

func (r *Runtime) MaxConcurrent() int {
	return r.maxConcurrent
}

func (r *Runtime) IsInFlight(tenantID int64, storeID int64) bool {
	key := runtimeStoreKey{tenantID: tenantID, storeID: storeID}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inFlight[key]
}

func (r *Runtime) withStoreLock(tenantID int64, storeID int64, fn func() error) error {
	key := runtimeStoreKey{tenantID: tenantID, storeID: storeID}
	lock := r.storeLock(key)
	lock.Lock()
	defer lock.Unlock()

	r.semaphore <- struct{}{}
	r.markInFlight(key, true)
	defer func() {
		r.markInFlight(key, false)
		<-r.semaphore
	}()
	return fn()
}

func (r *Runtime) storeLock(key runtimeStoreKey) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock := r.locks[key]
	if lock == nil {
		lock = &sync.Mutex{}
		r.locks[key] = lock
	}
	return lock
}

func (r *Runtime) markInFlight(key runtimeStoreKey, value bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inFlight[key] = value
}
