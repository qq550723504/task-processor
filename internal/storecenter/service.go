package storecenter

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

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

type StoreLimitReachedError struct{ Committed, Used, Reserved, Limit int64 }

func (e *StoreLimitReachedError) Error() string        { return "store limit reached" }
func (e *StoreLimitReachedError) Is(target error) bool { return target == ErrLimitReached }

type Service struct {
	repository Repository
	quota      listingsubscription.StoreQuotaLedger
	audit      AuditRepository
	now        func() time.Time
}

func NewService(repository Repository, quota listingsubscription.StoreQuotaLedger, audit AuditRepository, now func() time.Time) (*Service, error) {
	if isNilDependency(repository) || isNilDependency(quota) || isNilDependency(audit) || now == nil {
		return nil, errors.New("store service dependencies are required")
	}
	return &Service{repository: repository, quota: quota, audit: audit, now: now}, nil
}

func (s *Service) Create(ctx context.Context, request CreateStoreRequest) (CreateStoreResult, error) {
	request, err := normalizeCreateStoreRequest(request)
	if err != nil {
		return CreateStoreResult{}, err
	}
	reserved, err := s.quota.Reserve(ctx, listingsubscription.StoreQuotaReserveInput{OrganizationID: request.OrganizationID, RequestKey: request.IdempotencyKey, ActorSubject: request.ActorSubject})
	if err != nil {
		return CreateStoreResult{}, mapQuotaError(err)
	}
	allocation, err := validateReserveResult(request, reserved)
	if err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	transition := listingsubscription.StoreQuotaTransitionInput{OrganizationID: request.OrganizationID, AllocationID: allocation.AllocationID, StoreID: allocation.StoreID, RequestKey: request.IdempotencyKey, ActorSubject: request.ActorSubject}
	replayed := reserved.Existing

	if err := s.record(ctx, allocation, request, AuditActionQuotaReserved, AuditOutcomeSucceeded, nil, "", StoreStatusProvisioning, AuditFailureNone); err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
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

	store, err := s.repository.Get(ctx, request.OrganizationID, allocation.StoreID)
	if errors.Is(err, ErrNotFound) {
		switch allocation.Status {
		case listingsubscription.StoreQuotaReserved:
			store, err = NewStore(CreateStoreInput{ID: allocation.StoreID, OrganizationID: request.OrganizationID, ActorSubject: request.ActorSubject, Name: request.Name, Platform: request.Platform, Region: request.Region, ExternalStoreID: request.ExternalStoreID, CreateIdempotencyKey: request.IdempotencyKey, QuotaAllocationID: allocation.AllocationID, OccurredAt: s.utcNow()})
			if err != nil {
				return CreateStoreResult{}, err
			}
			var createdReplay bool
			store, createdReplay, err = s.repository.CreateOrReplay(ctx, request.OrganizationID, store)
			replayed = replayed || createdReplay
			if err != nil {
				createErr := err
				store, err = s.repository.Get(ctx, request.OrganizationID, allocation.StoreID)
				if errors.Is(err, ErrNotFound) {
					return s.compensateDefinitiveCreateFailure(ctx, allocation, transition, request, createErr)
				}
				if err != nil {
					_ = s.record(ctx, allocation, request, AuditActionStoreCreateUnknown, AuditOutcomeUnknown, nil, "", "", AuditFailureDependencyUnavailable)
					return CreateStoreResult{}, dependencyError(err)
				}
				replayed = true
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
	}
	if err := verifyStoreAllocation(store, request, allocation); err != nil {
		return CreateStoreResult{}, dependencyError(err)
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
		if err := s.releaseReservation(ctx, transition); err != nil {
			return dependencyError(err)
		}
		return nil
	default:
		return dependencyError(errors.New("terminal create failure has allocated quota"))
	}
}

func (s *Service) releaseReservation(ctx context.Context, input listingsubscription.StoreQuotaTransitionInput) error {
	released, err := s.quota.ReleaseReservation(ctx, input)
	if err != nil {
		return err
	}
	return validateTransitionAllocation(released.Allocation, input, listingsubscription.StoreQuotaReleased)
}

func (s *Service) record(ctx context.Context, allocation listingsubscription.StoreQuotaAllocation, request CreateStoreRequest, action AuditAction, outcome AuditOutcome, store *Store, previous, next LifecycleStatus, failure AuditFailureCode) error {
	storeID := allocation.StoreID
	if store != nil {
		storeID = store.ID()
	}
	_, _, err := s.audit.Record(ctx, newAuditEvent(request.OrganizationID, storeID, allocation.AllocationID, request.IdempotencyKey, action, outcome, request.ActorSubject, []string{"lifecycle_status", "quota_allocation_id"}, previous, next, failure, s.utcNow()))
	return err
}

func validateReserveResult(request CreateStoreRequest, reserved listingsubscription.StoreQuotaReserveResult) (listingsubscription.StoreQuotaAllocation, error) {
	a := reserved.Allocation
	if a.OrganizationID != request.OrganizationID || a.RequestKey != request.IdempotencyKey || a.AllocationID != reserved.AllocationID || a.StoreID != reserved.StoreID || a.Status == "" {
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
		if exceeded.Committed < 0 || exceeded.Reserved < 0 || exceeded.Limit <= 0 {
			return dependencyError(err)
		}
		return &StoreLimitReachedError{Committed: exceeded.Committed, Used: exceeded.Committed, Reserved: exceeded.Reserved, Limit: exceeded.Limit}
	}
	if errors.Is(err, listingsubscription.ErrSubscriptionRequired) {
		return err
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
