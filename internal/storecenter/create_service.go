package storecenter

import (
	"context"
	"errors"
	"fmt"
	"task-processor/internal/listingsubscription"
	"time"
)

const orphanedStoreReservationGracePeriod = 5 * time.Minute

func (s *Service) Create(ctx context.Context, request CreateStoreRequest) (CreateStoreResult, error) {
	request, err := normalizeCreateStoreRequest(request)
	if err != nil {
		return CreateStoreResult{}, err
	}
	// Reserve is durable, but the audit, Store write, and quota transition are
	// intentionally separate boundaries. Serialize the complete create state
	// machine for one organization/idempotency key so a replay cannot observe a
	// reservation while its owner is compensating a failed audit write.
	releaseCreate := s.createLocks.acquire(request.OrganizationID, request.IdempotencyKey)
	defer releaseCreate()

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
	if reserved.Existing && allocation.CreatedBy != "" {
		// A replay may be issued by another actor after the reservation was
		// persisted. Keep all recovery writes attributed to the durable owner.
		request.ActorSubject = allocation.CreatedBy
	}
	transition := listingsubscription.StoreQuotaTransitionInput{OrganizationID: request.OrganizationID, AllocationID: allocation.AllocationID, StoreID: allocation.StoreID, RequestKey: request.IdempotencyKey, ActorSubject: request.ActorSubject}
	releaseTransition := transition
	expectedReleaseUpdatedAt := allocation.UpdatedAt
	releaseTransition.ExpectedUpdatedAt = &expectedReleaseUpdatedAt
	candidate, err := s.createCandidate(request, allocation)
	if err != nil {
		return CreateStoreResult{}, dependencyError(err)
	}
	replayed := reserved.Existing

	if terminal, err := s.audit.Get(ctx, request.OrganizationID, request.IdempotencyKey, AuditActionStoreCreateFailed); err == nil {
		if !auditEventMatchesAllocation(*terminal, allocation.AllocationID, allocation.StoreID) {
			return CreateStoreResult{}, dependencyError(ErrAuditIdentityMismatch)
		}
		if err := s.finishReleasedFailure(ctx, allocation, releaseTransition, request, *terminal); err != nil {
			return CreateStoreResult{}, err
		}
		return CreateStoreResult{}, stableFailureFromAudit(*terminal)
	} else if !errors.Is(err, ErrNotFound) {
		return CreateStoreResult{}, dependencyError(err)
	}

	if err := s.record(ctx, allocation, request, AuditActionQuotaReserved, AuditOutcomeSucceeded, nil, "", StoreStatusProvisioning, AuditFailureNone); err != nil {
		if terminalErr := s.record(ctx, allocation, request, AuditActionStoreCreateFailed, AuditOutcomeFailed, nil, "", "", AuditFailureDependencyUnavailable); terminalErr != nil {
			return CreateStoreResult{}, dependencyError(terminalErr)
		}
		if releaseErr := s.releaseReservation(ctx, releaseTransition); releaseErr != nil {
			return CreateStoreResult{}, dependencyError(releaseErr)
		}
		return CreateStoreResult{}, dependencyError(err)
	}
	reservationLease := s.keepReservationLeaseAlive(ctx, transition, time.Minute)
	defer func() { _ = reservationLease.stop() }()

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
						refreshedTransition, refreshErr := s.stopReservationLeaseAndRefresh(ctx, reservationLease, releaseTransition)
						if refreshErr != nil {
							return CreateStoreResult{}, dependencyError(refreshErr)
						}
						return s.compensateDefinitiveCreateFailure(ctx, allocation, refreshedTransition, request, createErr)
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
			if allocation.Status == listingsubscription.StoreQuotaReleased && allocation.AllocatedAt != nil && allocation.ReleasedAt != nil {
				// The create key already produced a Store that was subsequently
				// deleted. It remains permanently consumed and must not be
				// interpreted as a transient dependency failure.
				return CreateStoreResult{}, ErrAlreadyExists
			}
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
		terminal, auditErr := s.audit.Get(ctx, organizationID, allocation.RequestKey, AuditActionStoreCreateFailed)
		if errors.Is(auditErr, ErrNotFound) {
			// A reservation without a durable terminal failure may still belong to
			// an in-flight create. Only the create state machine can release it.
			continue
		}
		if auditErr != nil {
			return recovered, auditErr
		}
		if terminal == nil || terminal.Outcome != AuditOutcomeFailed || !auditEventMatchesAllocation(*terminal, allocation.AllocationID, allocation.StoreID) {
			return recovered, ErrAuditIdentityMismatch
		}
		expectedUpdatedAt := allocation.UpdatedAt
		transition := listingsubscription.StoreQuotaTransitionInput{OrganizationID: organizationID, AllocationID: allocation.AllocationID, StoreID: allocation.StoreID, RequestKey: allocation.RequestKey, ActorSubject: allocation.CreatedBy, ExpectedUpdatedAt: &expectedUpdatedAt}
		if err := s.releaseReservation(ctx, transition); err != nil {
			if errors.Is(err, listingsubscription.ErrStoreQuotaStale) {
				continue
			}
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
}

func (s *Service) stopReservationLeaseAndRefresh(ctx context.Context, lease *reservationLease, fallback listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionInput, error) {
	if err := lease.stop(); err != nil {
		return listingsubscription.StoreQuotaTransitionInput{}, err
	}
	latest, err := s.quota.GetByRequestKey(ctx, fallback.OrganizationID, fallback.RequestKey)
	if err != nil {
		return listingsubscription.StoreQuotaTransitionInput{}, err
	}
	if latest == nil || latest.OrganizationID != fallback.OrganizationID || latest.AllocationID != fallback.AllocationID || latest.StoreID != fallback.StoreID || latest.RequestKey != fallback.RequestKey || latest.Status != listingsubscription.StoreQuotaReserved {
		return listingsubscription.StoreQuotaTransitionInput{}, errors.New("quota reservation changed before compensation")
	}
	refreshed := lease.transition()
	refreshed.OrganizationID = fallback.OrganizationID
	refreshed.AllocationID = fallback.AllocationID
	refreshed.StoreID = fallback.StoreID
	refreshed.RequestKey = fallback.RequestKey
	refreshed.ActorSubject = fallback.ActorSubject
	expectedUpdatedAt := latest.UpdatedAt.UTC()
	refreshed.ExpectedUpdatedAt = &expectedUpdatedAt
	return refreshed, nil
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
