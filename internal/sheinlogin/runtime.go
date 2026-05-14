package sheinlogin

import (
	"sync"
)

type Runtime struct {
	mu            sync.Mutex
	locks         map[int64]*sync.Mutex
	inFlight      map[int64]bool
	semaphore     chan struct{}
	maxConcurrent int
}

func NewRuntime(maxConcurrent int) *Runtime {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &Runtime{
		locks:         make(map[int64]*sync.Mutex),
		inFlight:      make(map[int64]bool),
		semaphore:     make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

func (r *Runtime) MaxConcurrent() int {
	return r.maxConcurrent
}

func (r *Runtime) IsInFlight(storeID int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inFlight[storeID]
}

func (r *Runtime) withStoreLock(storeID int64, fn func() error) error {
	lock := r.storeLock(storeID)
	lock.Lock()
	defer lock.Unlock()

	r.semaphore <- struct{}{}
	r.markInFlight(storeID, true)
	defer func() {
		r.markInFlight(storeID, false)
		<-r.semaphore
	}()
	return fn()
}

func (r *Runtime) storeLock(storeID int64) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock := r.locks[storeID]
	if lock == nil {
		lock = &sync.Mutex{}
		r.locks[storeID] = lock
	}
	return lock
}

func (r *Runtime) markInFlight(storeID int64, value bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inFlight[storeID] = value
}
