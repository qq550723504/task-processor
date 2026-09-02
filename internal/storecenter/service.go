package storecenter

import (
	"errors"
	"sync"
	"task-processor/internal/listingsubscription"
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
	Status         LifecycleStatus
}

type GetStoreRequest struct {
	OrganizationID string
	StoreID        string
}

type ResumeCreateStoreRequest struct {
	OrganizationID  string
	ActorSubject    string
	StoreID         string
	ExpectedVersion int64
}

type StoreProjection struct {
	Store            Store
	ConnectionStatus ConnectionStatus
}

type StoreQuotaProjection struct {
	Used     int64
	Reserved int64
	Limit    *int64
	Allowed  bool
	Reason   string
}

type ListStoresResult struct {
	Items    []StoreProjection
	Total    int64
	Page     int
	PageSize int
	Quota    StoreQuotaProjection
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

type StoreLimitReachedError struct{ Committed, Used, Reserved, Limit int64 }

func (e *StoreLimitReachedError) Error() string        { return "store limit reached" }
func (e *StoreLimitReachedError) Is(target error) bool { return target == ErrLimitReached }

type Service struct {
	repository  Repository
	quota       listingsubscription.StoreQuotaLedger
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

func NewService(repository Repository, quota listingsubscription.StoreQuotaLedger, audit AuditRepository, connections ConnectionStatusProvider, now func() time.Time) (*Service, error) {
	if isNilDependency(repository) || isNilDependency(quota) || isNilDependency(audit) || isNilDependency(connections) || now == nil {
		return nil, errors.New("store service dependencies are required")
	}
	return &Service{repository: repository, quota: quota, audit: audit, connections: connections, now: now}, nil
}
