package storecenter

import (
	"context"
	"errors"
	"time"
)

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
	apply                                      func(*Store, time.Time, string) (bool, error)
	matches                                    func(*Store) bool
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
	}, apply: func(store *Store, at time.Time, actor string) (bool, error) {
		return store.EditBasic(normalized.Name, normalized.Region, actor, at)
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
	return s.mutate(ctx, mutationRequest{organizationID: normalized.OrganizationID, actor: normalized.ActorSubject, storeID: normalized.StoreID, expectedVersion: normalized.ExpectedVersion, actionName: actionName, intentAuditAction: AuditActionStoreLifecycleStarted, auditAction: auditAction, fields: []string{"lifecycle_status"}, payloadFingerprint: hashTuple("store-lifecycle-request", actionName, string(from), string(to)), previous: from, next: to, apply: func(store *Store, at time.Time, actor string) (bool, error) {
		if store.LifecycleStatus() != from {
			return false, ErrInvalidTransition
		}
		if err := store.TransitionTo(to, actor, at); err != nil {
			return false, err
		}
		return true, nil
	}, matches: func(store *Store) bool { return store.LifecycleStatus() == to }})
}

func (s *Service) mutate(ctx context.Context, request mutationRequest) (StoreMutationResult, error) {
	release := s.locks.acquire(request.organizationID, request.storeID)
	defer release()

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
		if durableIntent != nil && store.Version() == request.expectedVersion {
			if validateMutationIntent(durableIntent, request, store, operationKey) != nil {
				return StoreMutationResult{}, dependencyError(ErrAuditIdentityMismatch)
			}
			// The intent is the durable owner of the in-flight mutation. A
			// cross-actor retry must use that owner for the aggregate write too,
			// otherwise UpdatedBy diverges from the audit provenance.
			request.actor = durableIntent.ActorSubject
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
		originalSnapshot := store.Snapshot()
		candidate, rehydrateErr := RehydrateStore(originalSnapshot)
		if rehydrateErr != nil {
			return StoreMutationResult{}, dependencyError(rehydrateErr)
		}
		applyActor := request.actor
		changed, applyErr := request.apply(candidate, s.monotonicNow(candidate.UpdatedAt()), applyActor)
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
					intentPrevious, intentNext = candidate.LifecycleStatus(), candidate.LifecycleStatus()
				}
				intent := newAuditEvent(request.organizationID, request.storeID, candidate.QuotaAllocationID(), operationKey, request.intentAuditAction, AuditOutcomeUnknown, applyActor, auditFields, intentPrevious, intentNext, AuditFailureNone, s.utcNow())
				intent.StoreVersion = request.expectedVersion
				intent.PayloadFingerprint = request.payloadFingerprint
				recordedIntent, _, intentErr := s.audit.Record(ctx, intent)
				if intentErr != nil {
					if errors.Is(intentErr, ErrAuditIdentityMismatch) {
						existingIntent, getErr := s.audit.Get(ctx, request.organizationID, operationKey, request.intentAuditAction)
						if getErr == nil && mutationIntentHasDifferentPayload(existingIntent, request, store, operationKey) {
							return StoreMutationResult{}, ErrVersionConflict
						}
					}
					return StoreMutationResult{}, dependencyError(intentErr)
				}
				if validateMutationIntent(&recordedIntent, request, candidate, operationKey) != nil {
					return StoreMutationResult{}, dependencyError(ErrAuditIdentityMismatch)
				}
				durableIntent = &recordedIntent
				auditFields = append([]string(nil), recordedIntent.SafeFieldNames...)
				if recordedIntent.ActorSubject != applyActor {
					candidate, rehydrateErr = RehydrateStore(originalSnapshot)
					if rehydrateErr != nil {
						return StoreMutationResult{}, dependencyError(rehydrateErr)
					}
					changed, applyErr = request.apply(candidate, s.monotonicNow(candidate.UpdatedAt()), recordedIntent.ActorSubject)
					if applyErr != nil {
						return StoreMutationResult{}, applyErr
					}
					if !changed {
						return StoreMutationResult{}, dependencyError(errors.New("durable mutation intent could not be applied"))
					}
				}
			}
			store = candidate
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
	if durableIntent != nil {
		event.ActorSubject = durableIntent.ActorSubject
	}
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

func mutationIntentHasDifferentPayload(event *AuditEvent, request mutationRequest, store *Store, operationKey string) bool {
	return event != nil && store != nil && event.OrganizationID == request.organizationID && event.StoreID == request.storeID && event.AllocationID == store.QuotaAllocationID() && event.RequestKey == operationKey && event.Action == request.intentAuditAction && event.Outcome == AuditOutcomeUnknown && event.FailureCode == AuditFailureNone && event.StoreVersion == request.expectedVersion && event.PayloadFingerprint != request.payloadFingerprint
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
	// Once later mutations have advanced the version, the current Store is not
	// historical evidence that this mutation reached expectedVersion+1. Only a
	// durable success result can authorize repairing the missing replay audit;
	// otherwise fail closed and let the caller reconcile the version conflict.
	if completed == nil {
		return StoreMutationResult{}, ErrVersionConflict
	}
	if completed.StoreID != request.storeID || completed.AllocationID != store.QuotaAllocationID() || completed.Outcome != AuditOutcomeSucceeded || completed.StoreVersion != request.expectedVersion+1 || completed.PayloadFingerprint != request.payloadFingerprint {
		return StoreMutationResult{}, dependencyError(ErrAuditIdentityMismatch)
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
