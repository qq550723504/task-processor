package storecenter

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/listingsubscription"
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
}

const orphanedStoreReservationGracePeriod = 5 * time.Minute

func NewService(repository Repository, quota listingsubscription.StoreQuotaLedger, audit AuditRepository, connections ConnectionStatusProvider, now func() time.Time) (*Service, error) {
	if isNilDependency(repository) || isNilDependency(quota) || isNilDependency(audit) || isNilDependency(connections) || now == nil {
		return nil, errors.New("store service dependencies are required")
	}
	return &Service{repository: repository, quota: quota, audit: audit, connections: connections, now: now}, nil
}

func (s *Service) Create(ctx context.Context, request CreateStoreRequest) (CreateStoreResult, error) {
	request, err := normalizeCreateStoreRequest(request)
	if err != nil {
		return CreateStoreResult{}, err
	}
	requestFingerprint := createQuotaRequestFingerprint(request)
	reserved, err := s.quota.Reserve(ctx, listingsubscription.StoreQuotaReserveInput{OrganizationID: request.OrganizationID, RequestKey: request.IdempotencyKey, ActorSubject: request.ActorSubject, RequestFingerprint: requestFingerprint})
	if err != nil && errors.Is(err, listingsubscription.ErrStoreQuotaExceeded) {
		if recovered, reconcileErr := s.reconcileOrphanedReservations(ctx, request.OrganizationID); reconcileErr != nil {
			return CreateStoreResult{}, dependencyError(reconcileErr)
		} else if recovered > 0 {
			reserved, err = s.quota.Reserve(ctx, listingsubscription.StoreQuotaReserveInput{OrganizationID: request.OrganizationID, RequestKey: request.IdempotencyKey, ActorSubject: request.ActorSubject, RequestFingerprint: requestFingerprint})
		}
	}
	if err != nil {
		return CreateStoreResult{}, mapQuotaError(err)
	}
	allocation, err := validateReserveResult(request, requestFingerprint, reserved)
	if err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	transition := listingsubscription.StoreQuotaTransitionInput{OrganizationID: request.OrganizationID, AllocationID: allocation.AllocationID, StoreID: allocation.StoreID, RequestKey: request.IdempotencyKey, ActorSubject: request.ActorSubject}
	candidate, err := s.createCandidate(request, allocation)
	if err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	replayed := reserved.Existing

	if terminal, err := s.audit.Get(ctx, request.OrganizationID, request.IdempotencyKey, AuditActionStoreCreateFailed); err == nil {
		if !auditEventMatchesAllocation(*terminal, allocation.AllocationID, allocation.StoreID) {
			return CreateStoreResult{}, dependencyError(ErrAuditIdentityMismatch)
		}
		if err := s.finishReleasedFailure(ctx, allocation, transition, request, *terminal); err != nil {
			return CreateStoreResult{}, err
		}
		return CreateStoreResult{}, stableFailureFromAudit(*terminal)
	} else if !errors.Is(err, ErrNotFound) {
		return CreateStoreResult{}, dependencyError(err)
	}

	if err := s.record(ctx, allocation, request, AuditActionQuotaReserved, AuditOutcomeSucceeded, nil, "", StoreStatusProvisioning, AuditFailureNone); err != nil {
		if releaseErr := s.releaseReservation(ctx, transition); releaseErr != nil {
			return CreateStoreResult{}, dependencyError(releaseErr)
		}
		if terminalErr := s.record(ctx, allocation, request, AuditActionStoreCreateFailed, AuditOutcomeFailed, nil, "", "", AuditFailureDependencyUnavailable); terminalErr != nil {
			return CreateStoreResult{}, dependencyError(terminalErr)
		}
		return CreateStoreResult{}, dependencyError(err)
	}
	stopReservationLease := s.keepReservationLeaseAlive(ctx, transition)
	defer stopReservationLease()

	store, err := s.repository.Get(ctx, request.OrganizationID, allocation.StoreID)
	verifyExistingCreate := false
	if errors.Is(err, ErrNotFound) {
		switch allocation.Status {
		case listingsubscription.StoreQuotaReserved:
			var createdReplay bool
			store, createdReplay, err = s.repository.CreateOrReplay(ctx, request.OrganizationID, candidate)
			replayed = replayed || createdReplay
			if err != nil {
				createErr := err
				store, err = s.repository.Get(ctx, request.OrganizationID, allocation.StoreID)
				if errors.Is(err, ErrNotFound) {
					if errors.Is(createErr, ErrAlreadyExists) {
						return s.compensateDefinitiveCreateFailure(ctx, allocation, transition, request, createErr)
					}
					_ = s.record(ctx, allocation, request, AuditActionStoreCreateUnknown, AuditOutcomeUnknown, nil, "", "", AuditFailureDependencyUnavailable)
					return CreateStoreResult{}, dependencyError(createErr)
				}
				if err != nil {
					_ = s.record(ctx, allocation, request, AuditActionStoreCreateUnknown, AuditOutcomeUnknown, nil, "", "", AuditFailureDependencyUnavailable)
					return CreateStoreResult{}, dependencyError(err)
				}
				replayed = true
				verifyExistingCreate = true
			}
		case listingsubscription.StoreQuotaAllocated, listingsubscription.StoreQuotaReleased:
			return CreateStoreResult{}, dependencyError(errors.New("quota allocation has no durable store"))
		default:
			return CreateStoreResult{}, dependencyError(errors.New("quota allocation status is invalid"))
		}
	} else if err != nil {
		return CreateStoreResult{}, dependencyError(err)
	} else {
		replayed = true
		verifyExistingCreate = true
	}
	if err := verifyStoreAllocation(store, request, allocation); err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	if verifyExistingCreate {
		verified, verifiedReplay, err := s.repository.CreateOrReplay(ctx, request.OrganizationID, candidate)
		if errors.Is(err, ErrAlreadyExists) {
			return CreateStoreResult{}, ErrAlreadyExists
		}
		if err != nil {
			return CreateStoreResult{}, dependencyError(err)
		}
		if err := verifyStoreAllocation(verified, request, allocation); err != nil {
			return CreateStoreResult{}, dependencyError(err)
		}
		store = verified
		replayed = replayed || verifiedReplay
	}
	if store.LifecycleStatus() == StoreStatusDeleting {
		return CreateStoreResult{}, dependencyError(errors.New("deleting store cannot be replayed"))
	}

	if err := s.record(ctx, allocation, request, AuditActionStoreCreated, AuditOutcomeSucceeded, store, "", StoreStatusProvisioning, AuditFailureNone); err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	if allocation.Status == listingsubscription.StoreQuotaReserved {
		// This truthful write-ahead event means a crash after it but before a
		// terminal quota outcome is conservatively resumed through idempotent
		// Commit rather than inventing a failed outcome.
		if err := s.record(ctx, allocation, request, AuditActionQuotaCommitStarted, AuditOutcomeUnknown, store, StoreStatusProvisioning, StoreStatusProvisioning, AuditFailureNone); err != nil {
			return CreateStoreResult{}, dependencyError(err)
		}
		committed, commitErr := s.quota.Commit(ctx, transition)
		if commitErr != nil {
			_ = s.record(ctx, allocation, request, AuditActionQuotaCommitFailed, AuditOutcomeFailed, store, StoreStatusProvisioning, StoreStatusProvisioning, AuditFailureDependencyUnavailable)
			return CreateStoreResult{}, dependencyError(commitErr)
		}
		allocation = committed.Allocation
		if err := validateTransitionAllocation(allocation, transition, listingsubscription.StoreQuotaAllocated); err != nil {
			return CreateStoreResult{}, dependencyError(err)
		}
	} else if allocation.Status != listingsubscription.StoreQuotaAllocated {
		return CreateStoreResult{}, dependencyError(errors.New("quota allocation cannot be committed"))
	}

	if store.LifecycleStatus() == StoreStatusProvisioning {
		expectedVersion := store.Version()
		if err := store.TransitionTo(StoreStatusActive, request.ActorSubject, s.monotonicNow(store.UpdatedAt())); err != nil {
			return CreateStoreResult{}, dependencyError(err)
		}
		if err := s.repository.Save(ctx, request.OrganizationID, store, expectedVersion); err != nil {
			resolved, readErr := s.repository.Get(ctx, request.OrganizationID, allocation.StoreID)
			if readErr != nil || resolved.LifecycleStatus() != StoreStatusActive || verifyStoreAllocation(resolved, request, allocation) != nil {
				return CreateStoreResult{}, dependencyError(err)
			}
			store = resolved
		}
	} else {
		replayed = true
	}
	if err := s.record(ctx, allocation, request, AuditActionStoreCreationCommitted, AuditOutcomeSucceeded, store, StoreStatusProvisioning, StoreStatusActive, AuditFailureNone); err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	return CreateStoreResult{Store: store, Replayed: replayed}, nil
}

const connectionStatusTimeout = 500 * time.Millisecond

func (s *Service) List(ctx context.Context, request ListStoresRequest) (ListStoresResult, error) {
	normalized, query, err := normalizeListStoresRequest(request)
	if err != nil {
		return ListStoresResult{}, err
	}
	page, err := s.repository.List(ctx, normalized.OrganizationID, query)
	if err != nil {
		return ListStoresResult{}, dependencyError(err)
	}
	if page.Total < 0 || int64(len(page.Stores)) > page.Total || len(page.Stores) > normalized.PageSize {
		return ListStoresResult{}, dependencyError(errors.New("store page is inconsistent"))
	}
	items := make([]StoreProjection, len(page.Stores))
	for i := range page.Stores {
		store, cloneErr := RehydrateStore(page.Stores[i].Snapshot())
		if cloneErr != nil || store.OrganizationID() != normalized.OrganizationID {
			return ListStoresResult{}, dependencyError(cloneErr)
		}
		items[i].Store = *store
	}
	s.projectConnections(ctx, items)
	summary, err := s.quota.Summary(ctx, normalized.OrganizationID)
	if err != nil {
		return ListStoresResult{}, dependencyError(err)
	}
	quota, err := validateQuotaSummary(normalized.OrganizationID, summary)
	if err != nil {
		return ListStoresResult{}, dependencyError(err)
	}
	return ListStoresResult{Items: items, Total: page.Total, Page: normalized.Page, PageSize: normalized.PageSize, Quota: quota}, nil
}

func (s *Service) Get(ctx context.Context, request GetStoreRequest) (StoreProjection, error) {
	normalized, err := normalizeGetStoreRequest(request)
	if err != nil {
		return StoreProjection{}, err
	}
	store, err := s.repository.Get(ctx, normalized.OrganizationID, normalized.StoreID)
	if errors.Is(err, ErrNotFound) {
		return StoreProjection{}, ErrNotFound
	}
	if err != nil {
		return StoreProjection{}, dependencyError(err)
	}
	if store == nil || store.OrganizationID() != normalized.OrganizationID || store.ID() != normalized.StoreID {
		return StoreProjection{}, dependencyError(errors.New("store read identity mismatch"))
	}
	return s.projectOne(ctx, store)
}

func (s *Service) ResumeCreate(ctx context.Context, request ResumeCreateStoreRequest) (CreateStoreResult, error) {
	normalized, err := normalizeResumeCreateStoreRequest(request)
	if err != nil {
		return CreateStoreResult{}, err
	}
	store, err := s.repository.Get(ctx, normalized.OrganizationID, normalized.StoreID)
	if errors.Is(err, ErrNotFound) {
		return CreateStoreResult{}, ErrNotFound
	}
	if err != nil || !matchesStoreScope(store, normalized.OrganizationID, normalized.StoreID) {
		return CreateStoreResult{}, dependencyError(err)
	}
	if store.Version() != normalized.ExpectedVersion {
		return CreateStoreResult{}, ErrVersionConflict
	}
	if store.LifecycleStatus() == StoreStatusActive {
		return CreateStoreResult{Store: store, Replayed: true}, nil
	}
	if store.LifecycleStatus() != StoreStatusProvisioning {
		return CreateStoreResult{}, ErrInvalidTransition
	}
	return s.Create(ctx, CreateStoreRequest{
		OrganizationID:  normalized.OrganizationID,
		ActorSubject:    normalized.ActorSubject,
		IdempotencyKey:  store.CreateIdempotencyKey(),
		Name:            store.Name(),
		Platform:        string(store.Platform()),
		Region:          store.Region(),
		ExternalStoreID: store.ExternalStoreID(),
	})
}

func (s *Service) Update(ctx context.Context, request UpdateStoreRequest) (StoreMutationResult, error) {
	normalized, err := normalizeUpdateStoreRequest(request)
	if err != nil {
		return StoreMutationResult{}, err
	}
	return s.mutate(ctx, mutationRequest{organizationID: normalized.OrganizationID, actor: normalized.ActorSubject, storeID: normalized.StoreID, expectedVersion: normalized.ExpectedVersion, actionName: "update", intentAuditAction: AuditActionStoreUpdateStarted, auditAction: AuditActionStoreUpdated, noOpAuditAction: AuditActionStoreUpdateNoOp, payloadFingerprint: hashTuple("store-update-request", normalized.Name, normalized.Region), fieldsFor: func(store *Store) []string {
		fields := make([]string, 0, 2)
		if store.Name() != normalized.Name {
			fields = append(fields, "name")
		}
		if store.Region() != normalized.Region {
			fields = append(fields, "region")
		}
		return fields
	}, apply: func(store *Store, at time.Time) (bool, error) {
		return store.EditBasic(normalized.Name, normalized.Region, normalized.ActorSubject, at)
	}, matches: func(store *Store) bool { return store.Name() == normalized.Name && store.Region() == normalized.Region }})
}

func (s *Service) Disable(ctx context.Context, request StoreLifecycleRequest) (StoreMutationResult, error) {
	return s.changeLifecycle(ctx, request, "disable", AuditActionStoreDisabled, StoreStatusActive, StoreStatusDisabled)
}

func (s *Service) Enable(ctx context.Context, request StoreLifecycleRequest) (StoreMutationResult, error) {
	return s.changeLifecycle(ctx, request, "enable", AuditActionStoreEnabled, StoreStatusDisabled, StoreStatusActive)
}

func (s *Service) changeLifecycle(ctx context.Context, request StoreLifecycleRequest, actionName string, auditAction AuditAction, from, to LifecycleStatus) (StoreMutationResult, error) {
	normalized, err := normalizeLifecycleRequest(request)
	if err != nil {
		return StoreMutationResult{}, err
	}
	return s.mutate(ctx, mutationRequest{organizationID: normalized.OrganizationID, actor: normalized.ActorSubject, storeID: normalized.StoreID, expectedVersion: normalized.ExpectedVersion, actionName: actionName, intentAuditAction: AuditActionStoreLifecycleStarted, auditAction: auditAction, fields: []string{"lifecycle_status"}, payloadFingerprint: hashTuple("store-lifecycle-request", actionName, string(from), string(to)), previous: from, next: to, apply: func(store *Store, at time.Time) (bool, error) {
		if store.LifecycleStatus() != from {
			return false, ErrInvalidTransition
		}
		if err := store.TransitionTo(to, normalized.ActorSubject, at); err != nil {
			return false, err
		}
		return true, nil
	}, matches: func(store *Store) bool { return store.LifecycleStatus() == to }})
}

type mutationRequest struct {
	organizationID, actor, storeID, actionName string
	expectedVersion                            int64
	auditAction                                AuditAction
	intentAuditAction                          AuditAction
	noOpAuditAction                            AuditAction
	fields                                     []string
	payloadFingerprint                         string
	fieldsFor                                  func(*Store) []string
	previous, next                             LifecycleStatus
	apply                                      func(*Store, time.Time) (bool, error)
	matches                                    func(*Store) bool
}

func (s *Service) mutate(ctx context.Context, request mutationRequest) (StoreMutationResult, error) {
	operationKey := deterministicMutationKey(request.organizationID, request.storeID, request.actionName, request.expectedVersion)
	store, err := s.repository.Get(ctx, request.organizationID, request.storeID)
	if errors.Is(err, ErrNotFound) {
		return StoreMutationResult{}, ErrNotFound
	}
	if err != nil || !matchesStoreScope(store, request.organizationID, request.storeID) {
		return StoreMutationResult{}, dependencyError(err)
	}
	var durableIntent *AuditEvent
	if request.intentAuditAction != "" {
		intent, intentErr := s.audit.Get(ctx, request.organizationID, operationKey, request.intentAuditAction)
		if intentErr == nil {
			durableIntent = intent
		} else if !errors.Is(intentErr, ErrNotFound) {
			return StoreMutationResult{}, dependencyError(intentErr)
		}
		if durableIntent != nil && store.Version() == request.expectedVersion && validateMutationIntent(durableIntent, request, store, operationKey) != nil {
			return StoreMutationResult{}, dependencyError(ErrAuditIdentityMismatch)
		}
	}
	replayed := false
	auditAction := request.auditAction
	auditFields := request.fields
	if store.Version() == request.expectedVersion+1 && request.matches(store) {
		if request.intentAuditAction != "" {
			if validateMutationIntent(durableIntent, request, store, operationKey) != nil {
				return StoreMutationResult{}, dependencyError(ErrAuditIdentityMismatch)
			}
			auditFields = append([]string(nil), durableIntent.SafeFieldNames...)
		}
		replayed = true
	} else if store.Version() > request.expectedVersion+1 && durableIntent != nil {
		return s.repairMutationAfterLaterVersion(ctx, request, operationKey, store, durableIntent)
	} else if store.Version() != request.expectedVersion {
		return StoreMutationResult{}, ErrVersionConflict
	} else {
		if request.fieldsFor != nil {
			auditFields = request.fieldsFor(store)
		}
		changed, applyErr := request.apply(store, s.monotonicNow(store.UpdatedAt()))
		if applyErr != nil {
			if errors.Is(applyErr, ErrInvalidTransition) {
				return StoreMutationResult{}, ErrInvalidTransition
			}
			return StoreMutationResult{}, applyErr
		}
		if changed {
			if request.intentAuditAction != "" {
				intentPrevious, intentNext := request.previous, request.next
				if request.actionName == "update" {
					intentPrevious, intentNext = store.LifecycleStatus(), store.LifecycleStatus()
				}
				intent := newAuditEvent(request.organizationID, request.storeID, store.QuotaAllocationID(), operationKey, request.intentAuditAction, AuditOutcomeUnknown, request.actor, auditFields, intentPrevious, intentNext, AuditFailureNone, s.utcNow())
				intent.StoreVersion = request.expectedVersion
				intent.PayloadFingerprint = request.payloadFingerprint
				recordedIntent, _, intentErr := s.audit.Record(ctx, intent)
				if intentErr != nil || validateMutationIntent(&recordedIntent, request, store, operationKey) != nil {
					return StoreMutationResult{}, dependencyError(intentErr)
				}
				durableIntent = &recordedIntent
				auditFields = append([]string(nil), recordedIntent.SafeFieldNames...)
			}
			if err := s.repository.Save(ctx, request.organizationID, store, request.expectedVersion); err != nil {
				resolved, readErr := s.repository.Get(ctx, request.organizationID, request.storeID)
				if readErr != nil || !matchesStoreScope(resolved, request.organizationID, request.storeID) {
					return StoreMutationResult{}, dependencyError(err)
				}
				if resolved.Version() == request.expectedVersion+1 && request.matches(resolved) {
					store = resolved
					replayed = true
				} else if resolved.Version() == request.expectedVersion {
					if errors.Is(err, ErrAlreadyExists) {
						return StoreMutationResult{}, ErrAlreadyExists
					}
					return StoreMutationResult{}, dependencyError(err)
				} else {
					return StoreMutationResult{}, ErrVersionConflict
				}
			}
		} else if request.noOpAuditAction != "" {
			auditAction = request.noOpAuditAction
			auditFields = nil
		}
	}
	previous, next := request.previous, request.next
	if request.actionName == "update" {
		previous, next = store.LifecycleStatus(), store.LifecycleStatus()
	}
	event := newAuditEvent(request.organizationID, request.storeID, store.QuotaAllocationID(), operationKey, auditAction, AuditOutcomeSucceeded, request.actor, auditFields, previous, next, AuditFailureNone, s.utcNow())
	event.StoreVersion = store.Version()
	event.PayloadFingerprint = request.payloadFingerprint
	_, auditReplayed, err := s.audit.Record(ctx, event)
	if err != nil {
		return StoreMutationResult{}, dependencyError(err)
	}
	projection, err := s.projectOne(ctx, store)
	if err != nil {
		return StoreMutationResult{}, err
	}
	return StoreMutationResult{Store: projection, Replayed: replayed || auditReplayed}, nil
}

func validateMutationIntent(event *AuditEvent, request mutationRequest, store *Store, operationKey string) error {
	if event == nil || store == nil || event.OrganizationID != request.organizationID || event.StoreID != request.storeID || event.AllocationID != store.QuotaAllocationID() || event.RequestKey != operationKey || event.Action != request.intentAuditAction || event.Outcome != AuditOutcomeUnknown || event.FailureCode != AuditFailureNone || event.StoreVersion != request.expectedVersion || event.PayloadFingerprint != request.payloadFingerprint {
		return ErrAuditIdentityMismatch
	}
	if request.actionName == "update" && (event.PreviousState != store.LifecycleStatus() || event.NewState != store.LifecycleStatus() || (!exactSafeFields(event.SafeFieldNames, "name") && !exactSafeFields(event.SafeFieldNames, "region") && !exactSafeFields(event.SafeFieldNames, "name", "region"))) {
		return ErrAuditIdentityMismatch
	}
	if request.actionName != "update" && (event.PreviousState != request.previous || event.NewState != request.next || !exactSafeFields(event.SafeFieldNames, "lifecycle_status")) {
		return ErrAuditIdentityMismatch
	}
	return nil
}

func validateMutationIntentForRepair(event *AuditEvent, request mutationRequest, store *Store, operationKey string) error {
	if event == nil || store == nil || event.OrganizationID != request.organizationID || event.StoreID != request.storeID || event.AllocationID != store.QuotaAllocationID() || event.RequestKey != operationKey || event.Action != request.intentAuditAction || event.Outcome != AuditOutcomeUnknown || event.FailureCode != AuditFailureNone || event.StoreVersion != request.expectedVersion || event.PayloadFingerprint != request.payloadFingerprint {
		return ErrAuditIdentityMismatch
	}
	if request.actionName == "update" {
		if event.PreviousState != event.NewState || (event.PreviousState != StoreStatusActive && event.PreviousState != StoreStatusDisabled) || (!exactSafeFields(event.SafeFieldNames, "name") && !exactSafeFields(event.SafeFieldNames, "region") && !exactSafeFields(event.SafeFieldNames, "name", "region")) {
			return ErrAuditIdentityMismatch
		}
		return nil
	}
	if event.PreviousState != request.previous || event.NewState != request.next || !exactSafeFields(event.SafeFieldNames, "lifecycle_status") {
		return ErrAuditIdentityMismatch
	}
	return nil
}

func (s *Service) repairMutationAfterLaterVersion(ctx context.Context, request mutationRequest, operationKey string, store *Store, intent *AuditEvent) (StoreMutationResult, error) {
	if validateMutationIntentForRepair(intent, request, store, operationKey) != nil {
		return StoreMutationResult{}, dependencyError(ErrAuditIdentityMismatch)
	}
	completed, err := s.audit.Get(ctx, request.organizationID, operationKey, request.auditAction)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return StoreMutationResult{}, dependencyError(err)
	}
	if completed != nil {
		if completed.StoreID != request.storeID || completed.AllocationID != store.QuotaAllocationID() || completed.Outcome != AuditOutcomeSucceeded || completed.StoreVersion != request.expectedVersion+1 || completed.PayloadFingerprint != request.payloadFingerprint {
			return StoreMutationResult{}, dependencyError(ErrAuditIdentityMismatch)
		}
	} else {
		event := newAuditEvent(request.organizationID, request.storeID, store.QuotaAllocationID(), operationKey, request.auditAction, AuditOutcomeSucceeded, intent.ActorSubject, intent.SafeFieldNames, intent.PreviousState, intent.NewState, AuditFailureNone, s.utcNow())
		event.StoreVersion = request.expectedVersion + 1
		event.PayloadFingerprint = request.payloadFingerprint
		if _, _, err := s.audit.Record(ctx, event); err != nil {
			return StoreMutationResult{}, dependencyError(err)
		}
	}
	projection, err := s.projectOne(ctx, store)
	if err != nil {
		return StoreMutationResult{}, err
	}
	return StoreMutationResult{Store: projection, Replayed: true}, nil
}

func matchesStoreScope(store *Store, organizationID, storeID string) bool {
	return store != nil && store.OrganizationID() == organizationID && store.ID() == storeID
}

func (s *Service) Delete(ctx context.Context, request DeleteStoreRequest) (DeleteStoreResult, error) {
	normalized, err := normalizeDeleteStoreRequest(request)
	if err != nil {
		return DeleteStoreResult{}, err
	}
	if completed, err := s.audit.Get(ctx, normalized.OrganizationID, normalized.OperationKey, AuditActionDeleteComplete); err == nil {
		if validateDeleteAudit(completed, normalized, AuditActionDeleteComplete) != nil {
			return DeleteStoreResult{}, dependencyError(ErrAuditIdentityMismatch)
		}
		return DeleteStoreResult{StoreID: normalized.StoreID, Version: completed.StoreVersion, Replayed: true}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return DeleteStoreResult{}, dependencyError(err)
	}

	store, getErr := s.repository.Get(ctx, normalized.OrganizationID, normalized.StoreID)
	if errors.Is(getErr, ErrNotFound) {
		deallocated, auditErr := s.audit.Get(ctx, normalized.OrganizationID, normalized.OperationKey, AuditActionQuotaDeallocated)
		if errors.Is(auditErr, ErrNotFound) {
			return DeleteStoreResult{}, ErrNotFound
		}
		if auditErr != nil || validateDeleteAudit(deallocated, normalized, AuditActionQuotaDeallocated) != nil {
			return DeleteStoreResult{}, dependencyError(auditErr)
		}
		version := deallocated.StoreVersion + 1
		if err := s.recordDeletePhase(ctx, normalized, deallocated.AllocationID, AuditActionDeleteComplete, StoreStatusDeleting, "", version); err != nil {
			return DeleteStoreResult{}, dependencyError(err)
		}
		return DeleteStoreResult{StoreID: normalized.StoreID, Version: version, Replayed: true}, nil
	}
	if getErr != nil || !matchesStoreScope(store, normalized.OrganizationID, normalized.StoreID) {
		return DeleteStoreResult{}, dependencyError(getErr)
	}
	replayed := false
	if store.LifecycleStatus() == StoreStatusDeleting {
		persistedOperationKey := store.DeleteOperationKey()
		if _, err := canonicalUUID(persistedOperationKey); err != nil {
			return DeleteStoreResult{}, dependencyError(err)
		}
		if persistedOperationKey != normalized.OperationKey {
			// The persisted key is the durable ownership record. A client may
			// lose its original key after a downstream failure, so recovery must
			// continue the durable operation instead of leaving the Store stuck
			// in deleting with its quota already released.
			normalized.OperationKey = persistedOperationKey
			replayed = true
		}
	}
	resumingSameOperation := store.LifecycleStatus() == StoreStatusDeleting && (store.Version() == normalized.ExpectedVersion || store.Version() == normalized.ExpectedVersion+1)
	if store.Version() != normalized.ExpectedVersion && !resumingSameOperation {
		return DeleteStoreResult{}, ErrVersionConflict
	}
	if store.LifecycleStatus() == StoreStatusProvisioning {
		return DeleteStoreResult{}, ErrInvalidTransition
	}
	previous := LifecycleStatus("")
	if store.LifecycleStatus() == StoreStatusDeleting {
		started, auditErr := s.audit.Get(ctx, normalized.OrganizationID, normalized.OperationKey, AuditActionDeleteStarted)
		if auditErr != nil || validateDeleteAudit(started, normalized, AuditActionDeleteStarted) != nil || started.AllocationID != store.QuotaAllocationID() {
			return DeleteStoreResult{}, dependencyError(auditErr)
		}
		previous = started.PreviousState
		replayed = true
	} else {
		previous = store.LifecycleStatus()
		if err := s.recordDeletePhase(ctx, normalized, store.QuotaAllocationID(), AuditActionDeleteStarted, previous, StoreStatusDeleting, store.Version()); err != nil {
			return DeleteStoreResult{}, dependencyError(err)
		}
		if err := store.BeginDelete(normalized.OperationKey, normalized.ActorSubject, s.monotonicNow(store.UpdatedAt())); err != nil {
			return DeleteStoreResult{}, err
		}
		if err := s.repository.Save(ctx, normalized.OrganizationID, store, normalized.ExpectedVersion); err != nil {
			resolved, readErr := s.repository.Get(ctx, normalized.OrganizationID, normalized.StoreID)
			if readErr == nil && matchesStoreScope(resolved, normalized.OrganizationID, normalized.StoreID) && resolved.LifecycleStatus() == StoreStatusDeleting && resolved.DeleteOperationKey() != normalized.OperationKey {
				return DeleteStoreResult{}, ErrInvalidTransition
			}
			if readErr != nil || !matchesStoreScope(resolved, normalized.OrganizationID, normalized.StoreID) || resolved.Version() != normalized.ExpectedVersion+1 || resolved.LifecycleStatus() != StoreStatusDeleting || resolved.DeleteOperationKey() != normalized.OperationKey {
				return DeleteStoreResult{}, dependencyError(err)
			}
			store = resolved
			replayed = true
		}
	}
	if err := s.recordDeletePhase(ctx, normalized, store.QuotaAllocationID(), AuditActionStoreMarkedDeleting, previous, StoreStatusDeleting, store.Version()); err != nil {
		return DeleteStoreResult{}, dependencyError(err)
	}
	transition := listingsubscription.StoreQuotaTransitionInput{OrganizationID: normalized.OrganizationID, AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), ActorSubject: normalized.ActorSubject}
	deallocated, err := s.quota.Deallocate(ctx, transition)
	if err != nil {
		return DeleteStoreResult{}, dependencyError(err)
	}
	if err := validateTransitionAllocation(deallocated.Allocation, transition, listingsubscription.StoreQuotaReleased); err != nil {
		return DeleteStoreResult{}, dependencyError(err)
	}
	if err := s.recordDeletePhase(ctx, normalized, store.QuotaAllocationID(), AuditActionQuotaDeallocated, StoreStatusDeleting, StoreStatusDeleting, store.Version()); err != nil {
		return DeleteStoreResult{}, dependencyError(err)
	}
	if err := s.repository.SoftDelete(ctx, normalized.OrganizationID, store.ID(), store.Version()); err != nil {
		_, readErr := s.repository.Get(ctx, normalized.OrganizationID, store.ID())
		if !errors.Is(readErr, ErrNotFound) {
			return DeleteStoreResult{}, dependencyError(err)
		}
	}
	version := store.Version() + 1
	if err := s.recordDeletePhase(ctx, normalized, store.QuotaAllocationID(), AuditActionDeleteComplete, StoreStatusDeleting, "", version); err != nil {
		return DeleteStoreResult{}, dependencyError(err)
	}
	return DeleteStoreResult{StoreID: store.ID(), Version: version, Replayed: replayed}, nil
}

func validateDeleteAudit(event *AuditEvent, request DeleteStoreRequest, action AuditAction) error {
	if event == nil || event.OrganizationID != request.OrganizationID || event.RequestKey != request.OperationKey || event.StoreID != request.StoreID || event.Action != action || event.FailureCode != AuditFailureNone || event.StoreVersion <= 0 {
		return ErrAuditIdentityMismatch
	}
	if _, err := canonicalUUID(event.AllocationID); err != nil {
		return ErrAuditIdentityMismatch
	}
	valid := false
	switch action {
	case AuditActionDeleteStarted:
		valid = event.Outcome == AuditOutcomeUnknown && (event.PreviousState == StoreStatusActive || event.PreviousState == StoreStatusDisabled) && event.NewState == StoreStatusDeleting && exactSafeFields(event.SafeFieldNames, "lifecycle_status") && (event.StoreVersion == request.ExpectedVersion || event.StoreVersion == request.ExpectedVersion-1)
	case AuditActionStoreMarkedDeleting:
		valid = event.Outcome == AuditOutcomeSucceeded && (event.PreviousState == StoreStatusActive || event.PreviousState == StoreStatusDisabled) && event.NewState == StoreStatusDeleting && exactSafeFields(event.SafeFieldNames, "lifecycle_status") && (event.StoreVersion == request.ExpectedVersion || event.StoreVersion == request.ExpectedVersion+1)
	case AuditActionQuotaDeallocated:
		valid = event.Outcome == AuditOutcomeSucceeded && event.PreviousState == StoreStatusDeleting && event.NewState == StoreStatusDeleting && exactSafeFields(event.SafeFieldNames, "quota_allocation_id") && (event.StoreVersion == request.ExpectedVersion || event.StoreVersion == request.ExpectedVersion+1)
	case AuditActionDeleteComplete:
		valid = event.Outcome == AuditOutcomeSucceeded && event.PreviousState == StoreStatusDeleting && event.NewState == "" && exactSafeFields(event.SafeFieldNames, "lifecycle_status") && (event.StoreVersion == request.ExpectedVersion+1 || event.StoreVersion == request.ExpectedVersion+2)
	}
	if !valid {
		return ErrAuditIdentityMismatch
	}
	return nil
}

func (s *Service) recordDeletePhase(ctx context.Context, request DeleteStoreRequest, allocationID string, action AuditAction, previous, next LifecycleStatus, version int64) error {
	outcome := AuditOutcomeSucceeded
	fields := []string{"lifecycle_status"}
	if action == AuditActionDeleteStarted {
		outcome = AuditOutcomeUnknown
	}
	if action == AuditActionQuotaDeallocated {
		fields = []string{"quota_allocation_id"}
	}
	event := newAuditEvent(request.OrganizationID, request.StoreID, allocationID, request.OperationKey, action, outcome, request.ActorSubject, fields, previous, next, AuditFailureNone, s.utcNow())
	event.StoreVersion = version
	_, _, err := s.audit.Record(ctx, event)
	return err
}

func exactSafeFields(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := make(map[string]bool, len(got))
	for _, field := range got {
		if seen[field] {
			return false
		}
		seen[field] = true
	}
	for _, field := range want {
		if !seen[field] {
			return false
		}
	}
	return true
}

func (s *Service) projectOne(ctx context.Context, store *Store) (StoreProjection, error) {
	if store == nil {
		return StoreProjection{}, dependencyError(errors.New("nil store"))
	}
	clone, err := RehydrateStore(store.Snapshot())
	if err != nil {
		return StoreProjection{}, dependencyError(err)
	}
	status := resolveConnectionStatus(ctx, s.connections, ConnectionStatusInput{OrganizationID: clone.OrganizationID(), StoreID: clone.ID(), Platform: clone.Platform(), ConnectionRef: clone.ConnectionRef()}, connectionStatusTimeout)
	return StoreProjection{Store: *clone, ConnectionStatus: status}, nil
}

func (s *Service) projectConnections(ctx context.Context, items []StoreProjection) {
	workers := len(items)
	if workers > 8 {
		workers = 8
	}
	jobs := make(chan int)
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range jobs {
				store := &items[index].Store
				items[index].ConnectionStatus = resolveConnectionStatus(ctx, s.connections, ConnectionStatusInput{OrganizationID: store.OrganizationID(), StoreID: store.ID(), Platform: store.Platform(), ConnectionRef: store.ConnectionRef()}, connectionStatusTimeout)
			}
		}()
	}
	for index := range items {
		jobs <- index
	}
	close(jobs)
	group.Wait()
}

func normalizeListStoresRequest(request ListStoresRequest) (ListStoresRequest, StoreListQuery, error) {
	organizationID, err := validateOpaqueIdentity("organization ID", request.OrganizationID, MaxOrganizationIDBytes)
	if err != nil || request.Page < 1 || request.PageSize < 1 || request.PageSize > 100 {
		return ListStoresRequest{}, StoreListQuery{}, errors.New("invalid store list request")
	}
	platform := Platform("")
	if request.Platform != "" {
		platform, err = normalizePlatform(request.Platform)
		if err != nil || string(platform) != request.Platform {
			return ListStoresRequest{}, StoreListQuery{}, errors.New("invalid store list platform")
		}
	}
	if request.Status != "" && !validLifecycleStatus(request.Status) {
		return ListStoresRequest{}, StoreListQuery{}, errors.New("invalid store list status")
	}
	request.OrganizationID = organizationID
	return request, StoreListQuery{Platform: platform, Status: request.Status, Page: request.Page, PageSize: request.PageSize}, nil
}

func normalizeGetStoreRequest(request GetStoreRequest) (GetStoreRequest, error) {
	var err error
	if request.OrganizationID, err = validateOpaqueIdentity("organization ID", request.OrganizationID, MaxOrganizationIDBytes); err != nil {
		return GetStoreRequest{}, err
	}
	if request.StoreID, err = canonicalUUID(request.StoreID); err != nil {
		return GetStoreRequest{}, err
	}
	return request, nil
}

func normalizeUpdateStoreRequest(request UpdateStoreRequest) (UpdateStoreRequest, error) {
	identity, err := normalizeMutationIdentity(request.OrganizationID, request.ActorSubject, request.StoreID, request.ExpectedVersion)
	if err != nil {
		return UpdateStoreRequest{}, err
	}
	request.OrganizationID, request.ActorSubject, request.StoreID = identity.OrganizationID, identity.ActorSubject, identity.StoreID
	if request.Name, err = normalizeUserValue("name", request.Name, MaxStoreNameCodePoints, true); err != nil {
		return UpdateStoreRequest{}, err
	}
	if request.Region, err = normalizeUserValue("region", request.Region, MaxStoreRegionCodePoints, true); err != nil {
		return UpdateStoreRequest{}, err
	}
	return request, nil
}

func normalizeLifecycleRequest(request StoreLifecycleRequest) (StoreLifecycleRequest, error) {
	identity, err := normalizeMutationIdentity(request.OrganizationID, request.ActorSubject, request.StoreID, request.ExpectedVersion)
	if err != nil {
		return StoreLifecycleRequest{}, err
	}
	request.OrganizationID, request.ActorSubject, request.StoreID = identity.OrganizationID, identity.ActorSubject, identity.StoreID
	return request, nil
}

func normalizeResumeCreateStoreRequest(request ResumeCreateStoreRequest) (ResumeCreateStoreRequest, error) {
	identity, err := normalizeMutationIdentity(request.OrganizationID, request.ActorSubject, request.StoreID, request.ExpectedVersion)
	if err != nil {
		return ResumeCreateStoreRequest{}, err
	}
	request.OrganizationID, request.ActorSubject, request.StoreID = identity.OrganizationID, identity.ActorSubject, identity.StoreID
	return request, nil
}

func normalizeDeleteStoreRequest(request DeleteStoreRequest) (DeleteStoreRequest, error) {
	identity, err := normalizeMutationIdentity(request.OrganizationID, request.ActorSubject, request.StoreID, request.ExpectedVersion)
	if err != nil {
		return DeleteStoreRequest{}, err
	}
	request.OrganizationID, request.ActorSubject, request.StoreID = identity.OrganizationID, identity.ActorSubject, identity.StoreID
	if request.OperationKey, err = canonicalUUID(request.OperationKey); err != nil {
		return DeleteStoreRequest{}, err
	}
	return request, nil
}

type mutationIdentity struct{ OrganizationID, ActorSubject, StoreID string }

func normalizeMutationIdentity(organizationID, actor, storeID string, expectedVersion int64) (mutationIdentity, error) {
	var err error
	if organizationID, err = validateOpaqueIdentity("organization ID", organizationID, MaxOrganizationIDBytes); err != nil {
		return mutationIdentity{}, err
	}
	if actor, err = validateOpaqueIdentity("actor subject", actor, MaxSubjectBytes); err != nil {
		return mutationIdentity{}, err
	}
	if storeID, err = canonicalUUID(storeID); err != nil || expectedVersion <= 0 {
		return mutationIdentity{}, errors.New("invalid versioned store request")
	}
	return mutationIdentity{organizationID, actor, storeID}, nil
}

func deterministicMutationKey(organizationID, storeID, action string, expectedVersion int64) string {
	name := organizationID + "\n" + storeID + "\n" + action + "\n" + strconv.FormatInt(expectedVersion, 10)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(name)).String()
}

func validateQuotaSummary(organizationID string, summary listingsubscription.StoreQuotaSummary) (StoreQuotaProjection, error) {
	if summary.OrganizationID != organizationID || summary.Committed < 0 || summary.Reserved < 0 {
		return StoreQuotaProjection{}, errors.New("quota summary identity or counts are invalid")
	}
	if summary.Limit == nil {
		if summary.Allowed || summary.Reason != "subscription_required" {
			return StoreQuotaProjection{}, errors.New("quota summary subscription state is invalid")
		}
	} else {
		if *summary.Limit <= 0 {
			return StoreQuotaProjection{}, errors.New("quota summary limit is invalid")
		}
		allowed := summary.Committed < *summary.Limit && summary.Reserved < *summary.Limit-summary.Committed
		if summary.Allowed != allowed || (allowed && summary.Reason != "") || (!allowed && summary.Reason != "store_limit_reached") {
			return StoreQuotaProjection{}, errors.New("quota summary availability is inconsistent")
		}
	}
	var limit *int64
	if summary.Limit != nil {
		value := *summary.Limit
		limit = &value
	}
	return StoreQuotaProjection{Used: summary.Committed, Reserved: summary.Reserved, Limit: limit, Allowed: summary.Allowed, Reason: summary.Reason}, nil
}

func (s *Service) createCandidate(request CreateStoreRequest, allocation listingsubscription.StoreQuotaAllocation) (*Store, error) {
	return NewStore(CreateStoreInput{ID: allocation.StoreID, OrganizationID: request.OrganizationID, ActorSubject: request.ActorSubject, Name: request.Name, Platform: request.Platform, Region: request.Region, ExternalStoreID: request.ExternalStoreID, CreateIdempotencyKey: request.IdempotencyKey, QuotaAllocationID: allocation.AllocationID, OccurredAt: s.utcNow()})
}

func normalizeCreateStoreRequest(request CreateStoreRequest) (CreateStoreRequest, error) {
	var err error
	if request.OrganizationID, err = validateOpaqueIdentity("organization ID", request.OrganizationID, MaxOrganizationIDBytes); err != nil {
		return CreateStoreRequest{}, err
	}
	if request.ActorSubject, err = validateOpaqueIdentity("actor subject", request.ActorSubject, MaxSubjectBytes); err != nil {
		return CreateStoreRequest{}, err
	}
	if request.IdempotencyKey, err = canonicalUUID(request.IdempotencyKey); err != nil {
		return CreateStoreRequest{}, fmt.Errorf("idempotency key: %w", err)
	}
	if request.Name, err = normalizeUserValue("name", request.Name, MaxStoreNameCodePoints, true); err != nil {
		return CreateStoreRequest{}, err
	}
	if _, err = normalizePlatform(request.Platform); err != nil {
		return CreateStoreRequest{}, err
	}
	request.Platform = string(PlatformShein)
	if request.Region, err = normalizeUserValue("region", request.Region, MaxStoreRegionCodePoints, true); err != nil {
		return CreateStoreRequest{}, err
	}
	if request.ExternalStoreID, err = normalizeUserValue("external store ID", request.ExternalStoreID, MaxExternalStoreIDCodePoints, false); err != nil {
		return CreateStoreRequest{}, err
	}
	return request, nil
}

func (s *Service) compensateDefinitiveCreateFailure(ctx context.Context, allocation listingsubscription.StoreQuotaAllocation, transition listingsubscription.StoreQuotaTransitionInput, request CreateStoreRequest, original error) (CreateStoreResult, error) {
	failure := mapRepositoryFailure(original)
	if err := s.record(ctx, allocation, request, AuditActionStoreCreateFailed, AuditOutcomeFailed, nil, "", "", auditFailureFor(failure)); err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	if err := s.requireNoStoreBeforeRelease(ctx, allocation, request.OrganizationID); err != nil {
		return CreateStoreResult{}, err
	}
	if err := s.releaseReservation(ctx, transition); err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	return CreateStoreResult{}, failure
}

func (s *Service) finishReleasedFailure(ctx context.Context, allocation listingsubscription.StoreQuotaAllocation, transition listingsubscription.StoreQuotaTransitionInput, request CreateStoreRequest, terminal AuditEvent) error {
	switch allocation.Status {
	case listingsubscription.StoreQuotaReleased:
		return nil
	case listingsubscription.StoreQuotaReserved:
		if err := s.requireNoStoreBeforeRelease(ctx, allocation, request.OrganizationID); err != nil {
			return err
		}
		if err := s.releaseReservation(ctx, transition); err != nil {
			return dependencyError(err)
		}
		return nil
	default:
		return dependencyError(errors.New("terminal create failure has allocated quota"))
	}
}

func (s *Service) requireNoStoreBeforeRelease(ctx context.Context, allocation listingsubscription.StoreQuotaAllocation, organizationID string) error {
	store, err := s.repository.Get(ctx, organizationID, allocation.StoreID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return dependencyError(err)
	}
	if err := verifyStoreAllocation(store, CreateStoreRequest{OrganizationID: organizationID, IdempotencyKey: allocation.RequestKey}, allocation); err != nil {
		return dependencyError(err)
	}
	return dependencyError(errors.New("store appeared before reservation release"))
}

func (s *Service) releaseReservation(ctx context.Context, input listingsubscription.StoreQuotaTransitionInput) error {
	released, err := s.quota.ReleaseReservation(ctx, input)
	if err != nil {
		return err
	}
	return validateTransitionAllocation(released.Allocation, input, listingsubscription.StoreQuotaReleased)
}

func (s *Service) reconcileOrphanedReservations(ctx context.Context, organizationID string) (int, error) {
	reconciler, ok := s.quota.(listingsubscription.StoreQuotaReservationReconciler)
	if !ok {
		return 0, nil
	}
	allocations, err := reconciler.ListReservedBefore(ctx, organizationID, s.utcNow().Add(-orphanedStoreReservationGracePeriod))
	if err != nil {
		return 0, err
	}
	recovered := 0
	for _, allocation := range allocations {
		_, getErr := s.repository.Get(ctx, organizationID, allocation.StoreID)
		if getErr == nil {
			continue
		}
		if !errors.Is(getErr, ErrNotFound) {
			// A transient read cannot prove that the reservation is orphaned.
			continue
		}
		transition := listingsubscription.StoreQuotaTransitionInput{OrganizationID: organizationID, AllocationID: allocation.AllocationID, StoreID: allocation.StoreID, RequestKey: allocation.RequestKey, ActorSubject: allocation.CreatedBy}
		if err := s.releaseReservation(ctx, transition); err != nil {
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
}

func (s *Service) keepReservationLeaseAlive(ctx context.Context, input listingsubscription.StoreQuotaTransitionInput) func() {
	leaseCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				_, _ = s.quota.RenewReservation(leaseCtx, input)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (s *Service) record(ctx context.Context, allocation listingsubscription.StoreQuotaAllocation, request CreateStoreRequest, action AuditAction, outcome AuditOutcome, store *Store, previous, next LifecycleStatus, failure AuditFailureCode) error {
	storeID := allocation.StoreID
	if store != nil {
		storeID = store.ID()
	}
	_, _, err := s.audit.Record(ctx, newAuditEvent(request.OrganizationID, storeID, allocation.AllocationID, request.IdempotencyKey, action, outcome, request.ActorSubject, []string{"lifecycle_status", "quota_allocation_id"}, previous, next, failure, s.utcNow()))
	return err
}

func validateReserveResult(request CreateStoreRequest, requestFingerprint string, reserved listingsubscription.StoreQuotaReserveResult) (listingsubscription.StoreQuotaAllocation, error) {
	a := reserved.Allocation
	if a.OrganizationID != request.OrganizationID || a.RequestKey != request.IdempotencyKey || a.RequestFingerprint != requestFingerprint || a.AllocationID != reserved.AllocationID || a.StoreID != reserved.StoreID || a.Status == "" {
		return listingsubscription.StoreQuotaAllocation{}, errors.New("quota reservation identity mismatch")
	}
	if _, err := canonicalUUID(a.AllocationID); err != nil {
		return listingsubscription.StoreQuotaAllocation{}, err
	}
	if _, err := canonicalUUID(a.StoreID); err != nil {
		return listingsubscription.StoreQuotaAllocation{}, err
	}
	return a, nil
}
func createQuotaRequestFingerprint(request CreateStoreRequest) string {
	return hashTuple("store-create-reservation", request.OrganizationID, request.Name, request.Platform, request.Region, request.ExternalStoreID)
}

func validateTransitionAllocation(a listingsubscription.StoreQuotaAllocation, input listingsubscription.StoreQuotaTransitionInput, status listingsubscription.StoreQuotaAllocationStatus) error {
	if a.OrganizationID != input.OrganizationID || a.AllocationID != input.AllocationID || a.StoreID != input.StoreID || a.RequestKey != input.RequestKey || a.Status != status {
		return errors.New("quota transition identity mismatch")
	}
	return nil
}
func verifyStoreAllocation(store *Store, request CreateStoreRequest, allocation listingsubscription.StoreQuotaAllocation) error {
	if store == nil || store.OrganizationID() != request.OrganizationID || store.ID() != allocation.StoreID || store.QuotaAllocationID() != allocation.AllocationID || store.CreateIdempotencyKey() != request.IdempotencyKey {
		return errors.New("store and quota allocation identity mismatch")
	}
	return nil
}
func mapQuotaError(err error) error {
	var exceeded *listingsubscription.StoreQuotaExceededError
	if errors.As(err, &exceeded) {
		if exceeded == nil || exceeded.Committed < 0 || exceeded.Reserved < 0 || exceeded.Limit <= 0 || (exceeded.Committed < exceeded.Limit && exceeded.Reserved < exceeded.Limit-exceeded.Committed) {
			return dependencyError(err)
		}
		return &StoreLimitReachedError{Committed: exceeded.Committed, Used: exceeded.Committed, Reserved: exceeded.Reserved, Limit: exceeded.Limit}
	}
	if errors.Is(err, listingsubscription.ErrSubscriptionRequired) {
		return fmt.Errorf("%w", listingsubscription.ErrSubscriptionRequired)
	}
	if errors.Is(err, listingsubscription.ErrStoreQuotaIdentityMismatch) {
		return ErrAlreadyExists
	}
	return dependencyError(err)
}
func mapRepositoryFailure(err error) error {
	if errors.Is(err, ErrAlreadyExists) {
		return ErrAlreadyExists
	}
	return dependencyError(err)
}
func dependencyError(_ error) error {
	return fmt.Errorf("%w", ErrDependencyUnavailable)
}
func (s *Service) utcNow() time.Time { return s.now().UTC() }
func (s *Service) monotonicNow(previous time.Time) time.Time {
	now := s.utcNow()
	if now.Before(previous) {
		return previous
	}
	return now
}
func isNilDependency(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	return (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface || v.Kind() == reflect.Map || v.Kind() == reflect.Func || v.Kind() == reflect.Slice) && v.IsNil()
}
