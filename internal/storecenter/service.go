package storecenter

import (
	"errors"
	"sync"
	"time"
)

type CreateStoreRequest struct {
	OrganizationID  string
	ActorSubject    string
	IdempotencyKey  string
	Name            string
	Platform        string
	Region          string
	ExternalStoreID string
}

type CreateStoreResult struct {
	Store    *Store
	Replayed bool
}

type ListStoresRequest struct {
	OrganizationID string
	Page           int
	PageSize       int
	Platform       string
	Status         RecordStatus
}

type GetStoreRequest struct {
	OrganizationID string
	StoreID        string
}

type StoreProjection struct {
	Store            Store
	ConnectionStatus ConnectionStatus
}

type ListStoresResult struct {
	Items    []StoreProjection
	Total    int64
	Page     int
	PageSize int
}

type UpdateStoreRequest struct {
	OrganizationID  string
	ActorSubject    string
	StoreID         string
	ExpectedVersion int64
	Name            string
	Region          string
}

type StoreLifecycleRequest struct {
	OrganizationID  string
	ActorSubject    string
	StoreID         string
	ExpectedVersion int64
}

type StoreMutationResult struct {
	Store    StoreProjection
	Replayed bool
}

type DeleteStoreRequest struct {
	OrganizationID  string
	ActorSubject    string
	StoreID         string
	ExpectedVersion int64
	OperationKey    string
}

type DeleteStoreResult struct {
	StoreID  string
	Version  int64
	Replayed bool
}

type Service struct {
	repository  Repository
	audit       AuditRepository
	connections ConnectionStatusProvider
	now         func() time.Time
	locks       mutationLockRegistry
	createLocks mutationLockRegistry
}

type mutationLockRegistry struct {
	mu    sync.Mutex
	locks map[mutationLockKey]*mutationLock
}

type mutationLockKey struct {
	organizationID string
	storeID        string
}

type mutationLock struct {
	mu   sync.Mutex
	refs int
}

func (r *mutationLockRegistry) acquire(organizationID, storeID string) func() {
	key := mutationLockKey{organizationID: organizationID, storeID: storeID}
	r.mu.Lock()
	if r.locks == nil {
		r.locks = make(map[mutationLockKey]*mutationLock)
	}
	lock := r.locks[key]
	if lock == nil {
		lock = &mutationLock{}
		r.locks[key] = lock
	}
	lock.refs++
	r.mu.Unlock()

	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		r.mu.Lock()
		lock.refs--
		if lock.refs == 0 {
			delete(r.locks, key)
		}
		r.mu.Unlock()
	}
}

func NewService(repository Repository, audit AuditRepository, connections ConnectionStatusProvider, now func() time.Time) (*Service, error) {
	if isNilDependency(repository) || isNilDependency(audit) || isNilDependency(connections) || now == nil {
		return nil, errors.New("store service dependencies are required")
	}
	return &Service{repository: repository, audit: audit, connections: connections, now: now}, nil
}
