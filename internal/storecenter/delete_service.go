package storecenter

import (
	"context"
	"errors"
	"task-processor/internal/listingsubscription"
)

func (s *Service) Delete(ctx context.Context, request DeleteStoreRequest) (DeleteStoreResult, error) {
	normalized, err := normalizeDeleteStoreRequest(request)
	if err != nil {
		return DeleteStoreResult{}, err
	}
	release := s.locks.acquire(normalized.OrganizationID, normalized.StoreID)
	defer release()
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
		if deallocated, auditErr := s.audit.GetByStoreID(ctx, normalized.OrganizationID, normalized.StoreID, AuditActionQuotaDeallocated); auditErr == nil {
			recovery := normalized
			recovery.OperationKey = deallocated.RequestKey
			recovery.ActorSubject = deallocated.ActorSubject
			if validateDeleteAudit(deallocated, recovery, AuditActionQuotaDeallocated) != nil {
				return DeleteStoreResult{}, dependencyError(ErrAuditIdentityMismatch)
			}
			if _, completeErr := s.audit.Get(ctx, recovery.OrganizationID, recovery.OperationKey, AuditActionDeleteComplete); completeErr == nil {
				// A different operation key arriving after a completed delete is
				// not a replay of the original ownership claim. The normal
				// same-key lookup above already handles legitimate idempotency.
				return DeleteStoreResult{}, ErrNotFound
			} else if !errors.Is(completeErr, ErrNotFound) {
				return DeleteStoreResult{}, dependencyError(completeErr)
			}
			version := deallocated.StoreVersion + 1
			if err := s.recordDeletePhase(ctx, recovery, deallocated.AllocationID, AuditActionDeleteComplete, StoreStatusDeleting, "", version); err != nil {
				return DeleteStoreResult{}, dependencyError(err)
			}
			return DeleteStoreResult{StoreID: recovery.StoreID, Version: version, Replayed: true}, nil
		} else if !errors.Is(auditErr, ErrNotFound) {
			return DeleteStoreResult{}, dependencyError(auditErr)
		}
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
			if store.Version() != normalized.ExpectedVersion {
				return DeleteStoreResult{}, ErrVersionConflict
			}
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
		normalized.ActorSubject = started.ActorSubject
		previous = started.PreviousState
		replayed = true
	} else {
		previous = store.LifecycleStatus()
		if started, auditErr := s.audit.Get(ctx, normalized.OrganizationID, normalized.OperationKey, AuditActionDeleteStarted); auditErr == nil {
			if validateDeleteAudit(started, normalized, AuditActionDeleteStarted) != nil || started.AllocationID != store.QuotaAllocationID() {
				return DeleteStoreResult{}, dependencyError(ErrAuditIdentityMismatch)
			}
			normalized.ActorSubject = started.ActorSubject
			previous = started.PreviousState
			replayed = true
		} else if !errors.Is(auditErr, ErrNotFound) {
			return DeleteStoreResult{}, dependencyError(auditErr)
		} else if err := s.recordDeletePhase(ctx, normalized, store.QuotaAllocationID(), AuditActionDeleteStarted, previous, StoreStatusDeleting, store.Version()); err != nil {
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
