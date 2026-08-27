package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/imageagent"
)

func (r *memoryRepository) InitializeRun(_ context.Context, input imageagent.ProjectionInitialization) (imageagent.RunProjection, error) {
	prepared, fingerprint, err := prepareInitialization(input)
	if err != nil {
		return imageagent.RunProjection{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(prepared.Scope)
	if commits := r.projectionCommits[key]; commits != nil {
		if existing, ok := commits[prepared.CommitID]; ok {
			if existing.fingerprint != fingerprint {
				return imageagent.RunProjection{}, imageagent.ErrRevisionConflict
			}
			return cloneProjection(existing.snapshot), nil
		}
	}
	if _, exists := r.runs[key]; exists {
		return imageagent.RunProjection{}, imageagent.ErrRevisionConflict
	}
	for _, existing := range r.runs {
		if existing.TenantID == prepared.Run.TenantID && existing.UserID == prepared.Run.UserID && existing.IdempotencyKey == prepared.Run.IdempotencyKey {
			return imageagent.RunProjection{}, imageagent.ErrRevisionConflict
		}
	}
	prepared = materializeRepositoryCatalogTimestamp(prepared, time.Now().UTC())
	r.runs[key] = cloneRun(prepared.Run)
	r.plans[key] = map[int64]imageagent.Plan{prepared.Plan.Revision: clonePlan(prepared.Plan)}
	for _, slot := range prepared.Plan.Slots {
		r.slots[slotKey(prepared.Scope, prepared.Plan.Revision, slot.ID)] = slotResultRecord{slot: cloneSlot(slot)}
	}
	r.catalogs[key] = cloneCatalog(prepared.Catalog)
	r.projections[key] = cloneProjection(prepared.Snapshot)
	r.events[key] = append(r.events[key], imageagent.RunEvent{TenantID: prepared.Scope.TenantID, OwnerUserID: prepared.Scope.OwnerUserID, RunID: prepared.Scope.RunID, Type: prepared.EventType, Cursor: 1, ProjectionVersion: 1, Payload: append(json.RawMessage(nil), prepared.EventPayload...)})
	r.projectionCommits[key] = map[string]projectionCommitMemory{prepared.CommitID: {fingerprint: fingerprint, version: 1, snapshot: cloneProjection(prepared.Snapshot)}}
	return cloneProjection(prepared.Snapshot), nil
}

func (r *memoryRepository) GetProjection(_ context.Context, scope imageagent.RunScope) (imageagent.RunProjection, error) {
	if err := validateScope(scope); err != nil {
		return imageagent.RunProjection{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[scopeKey(scope)]; !exists {
		return imageagent.RunProjection{}, imageagent.ErrRunNotFound
	}
	projection, exists := r.projections[scopeKey(scope)]
	if !exists {
		return imageagent.RunProjection{}, imageagent.ErrProjectionSnapshotMissing
	}
	result := cloneProjection(projection)
	if err := imageagent.ValidateProjectionSnapshot(scope, result); err != nil {
		return imageagent.RunProjection{}, err
	}
	return result, nil
}

func (r *memoryRepository) CommitProjection(_ context.Context, input imageagent.ProjectionCommit) (imageagent.RunProjection, error) {
	fingerprint, err := projectionCommitFingerprint(input)
	if err != nil {
		return imageagent.RunProjection{}, err
	}
	if err := validateScope(input.Scope); err != nil {
		return imageagent.RunProjection{}, err
	}
	if strings.TrimSpace(input.CommitID) == "" || strings.TrimSpace(input.EventType) == "" {
		return imageagent.RunProjection{}, fmt.Errorf("projection commit ID and event type are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(input.Scope)
	if _, exists := r.runs[key]; !exists {
		return imageagent.RunProjection{}, imageagent.ErrRunNotFound
	}
	if existing, ok := r.projectionCommits[key][input.CommitID]; ok {
		if existing.fingerprint != fingerprint {
			return imageagent.RunProjection{}, imageagent.ErrRevisionConflict
		}
		return cloneProjection(existing.snapshot), nil
	}
	current, exists := r.projections[key]
	if !exists {
		return imageagent.RunProjection{}, imageagent.ErrProjectionSnapshotMissing
	}
	if current.ProjectionVersion != input.ExpectedProjectionVersion {
		return imageagent.RunProjection{}, imageagent.ErrRevisionConflict
	}
	if err := validateProjectionPreconditions(current, r.runs[key], input); err != nil {
		return imageagent.RunProjection{}, err
	}
	if err := validateProjectionMutation(current, input); err != nil {
		return imageagent.RunProjection{}, err
	}
	rollback := r.captureMemoryProjectionState()
	if err := r.applyMemoryProjectionMutation(input); err != nil {
		r.restoreMemoryProjectionState(rollback)
		return imageagent.RunProjection{}, err
	}
	next := cloneProjection(input.Snapshot)
	next.LastEventID = current.ProjectionVersion + 1
	next.ProjectionVersion = next.LastEventID
	next.Actions = imageagent.AllowedActions(next.Run)
	if err := imageagent.ValidateProjectionSnapshot(input.Scope, next); err != nil {
		r.restoreMemoryProjectionState(rollback)
		return imageagent.RunProjection{}, err
	}
	r.projections[key] = cloneProjection(next)
	event := imageagent.RunEvent{TenantID: input.Scope.TenantID, OwnerUserID: input.Scope.OwnerUserID, RunID: input.Scope.RunID, Type: input.EventType, Cursor: next.ProjectionVersion, ProjectionVersion: next.ProjectionVersion, Payload: append(json.RawMessage(nil), input.EventPayload...)}
	r.events[key] = append(r.events[key], event)
	if r.projectionCommits[key] == nil {
		r.projectionCommits[key] = map[string]projectionCommitMemory{}
	}
	r.projectionCommits[key][input.CommitID] = projectionCommitMemory{fingerprint: fingerprint, version: next.ProjectionVersion, snapshot: cloneProjection(next)}
	return cloneProjection(next), nil
}

type memoryProjectionState struct {
	runs     map[string]imageagent.Run
	plans    map[string]map[int64]imageagent.Plan
	slots    map[string]slotResultRecord
	attempts map[string]imageagent.StepAttempt
	byNumber map[string]imageagent.StepAttempt
}

func (r *memoryRepository) captureMemoryProjectionState() memoryProjectionState {
	state := memoryProjectionState{
		runs:     make(map[string]imageagent.Run, len(r.runs)),
		plans:    make(map[string]map[int64]imageagent.Plan, len(r.plans)),
		slots:    make(map[string]slotResultRecord, len(r.slots)),
		attempts: make(map[string]imageagent.StepAttempt, len(r.attempts)),
		byNumber: make(map[string]imageagent.StepAttempt, len(r.byNumber)),
	}
	for key, run := range r.runs {
		state.runs[key] = cloneRun(run)
	}
	for key, revisions := range r.plans {
		state.plans[key] = make(map[int64]imageagent.Plan, len(revisions))
		for revision, plan := range revisions {
			state.plans[key][revision] = clonePlan(plan)
		}
	}
	for key, slot := range r.slots {
		state.slots[key] = slotResultRecord{slot: cloneSlot(slot.slot), result: cloneSlotResult(slot.result)}
	}
	for key, attempt := range r.attempts {
		state.attempts[key] = attempt
	}
	for key, attempt := range r.byNumber {
		state.byNumber[key] = attempt
	}
	return state
}

func (r *memoryRepository) restoreMemoryProjectionState(state memoryProjectionState) {
	r.runs = state.runs
	r.plans = state.plans
	r.slots = state.slots
	r.attempts = state.attempts
	r.byNumber = state.byNumber
}

func (r *memoryRepository) applyMemoryProjectionMutation(input imageagent.ProjectionCommit) error {
	key := scopeKey(input.Scope)
	if input.RunMutation != nil {
		run := r.runs[key]
		if run.Version != input.ExpectedRunVersion {
			return imageagent.ErrRevisionConflict
		}
		run.Status = input.RunMutation.Status
		run.CurrentNode = input.RunMutation.CurrentNode
		run.ActivePlanRevision = input.RunMutation.ActivePlanRevision
		run.Block = cloneBlock(input.RunMutation.Block)
		run.Version++
		r.runs[key] = run
	}
	if input.PlanMutation != nil {
		run := r.runs[key]
		mutation := input.PlanMutation
		if r.plans[key] == nil {
			r.plans[key] = map[int64]imageagent.Plan{}
		}
		if _, exists := r.plans[key][mutation.Plan.Revision]; exists {
			return imageagent.ErrRevisionConflict
		}
		r.plans[key][mutation.Plan.Revision] = clonePlan(mutation.Plan)
		for _, slot := range mutation.Plan.Slots {
			r.slots[slotKey(input.Scope, mutation.Plan.Revision, slot.ID)] = slotResultRecord{slot: cloneSlot(slot)}
		}
		run.ActivePlanRevision = mutation.Plan.Revision
		r.runs[key] = run
	}
	if input.SlotMutation != nil {
		mutation := input.SlotMutation
		run := r.runs[key]
		if run.ActivePlanRevision != mutation.PlanRevision {
			return imageagent.ErrRevisionConflict
		}
		storedKey := slotKey(input.Scope, mutation.PlanRevision, mutation.Result.SlotID)
		stored, exists := r.slots[storedKey]
		if !exists {
			return imageagent.ErrRevisionConflict
		}
		if stored.result.Attempt >= mutation.Result.Attempt && !sameSlotResult(stored.result, mutation.Result) {
			return imageagent.ErrRevisionConflict
		}
		stored.result = cloneSlotResult(mutation.Result)
		stored.slot.Status = mutation.Result.Status
		r.slots[storedKey] = stored
		attempt := mutation.Attempt
		if err := validateAttempt(attempt); err != nil {
			return err
		}
		if _, exists := r.attempts[attemptKey(attempt)]; exists {
			return imageagent.ErrRevisionConflict
		}
		r.attempts[attemptKey(attempt)] = attempt
		r.byNumber[attemptNumberKey(attempt)] = attempt
	}
	return nil
}

func (r *gormRepository) InitializeRun(ctx context.Context, input imageagent.ProjectionInitialization) (imageagent.RunProjection, error) {
	prepared, fingerprint, err := prepareInitialization(input)
	if err != nil {
		return imageagent.RunProjection{}, err
	}
	var result imageagent.RunProjection
	err = withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		if existing, found, err := findProjectionCommit(ctx, tx, prepared.Scope, prepared.CommitID); err != nil {
			return err
		} else if found {
			if existing.Fingerprint != fingerprint {
				return imageagent.ErrRevisionConflict
			}
			return decodeProjection(existing.SnapshotJSON, &result)
		}
		prepared = materializeRepositoryCatalogTimestamp(prepared, time.Now().UTC())
		runRow, err := runToRecord(prepared.Run)
		if err != nil {
			return err
		}
		if err := tx.Create(&runRow).Error; err != nil {
			return mapCreateConflict(err)
		}
		manifest := catalogManifestRow(prepared.Scope, prepared.Catalog)
		if err := tx.Create(&manifest).Error; err != nil {
			return fmt.Errorf("create image agent catalog manifest: %w", err)
		}
		rows, err := catalogRows(prepared.Scope, prepared.Catalog)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return fmt.Errorf("create image agent catalog: %w", err)
			}
		}
		planRow, slotRows, err := planToRecords(prepared.Scope, prepared.Plan)
		if err != nil {
			return err
		}
		if err := tx.Create(&planRow).Error; err != nil {
			return fmt.Errorf("create image agent plan: %w", err)
		}
		if len(slotRows) > 0 {
			if err := tx.Create(&slotRows).Error; err != nil {
				return fmt.Errorf("create image agent slots: %w", err)
			}
		}
		snapshotJSON, err := json.Marshal(prepared.Snapshot)
		if err != nil {
			return err
		}
		if err := tx.Create(&projectionRecord{TenantID: prepared.Scope.TenantID, OwnerUserID: prepared.Scope.OwnerUserID, RunID: prepared.Scope.RunID, Version: 1, SnapshotJSON: snapshotJSON}).Error; err != nil {
			return err
		}
		if err := tx.Create(&eventRecord{TenantID: prepared.Scope.TenantID, OwnerUserID: prepared.Scope.OwnerUserID, RunID: prepared.Scope.RunID, Type: prepared.EventType, Cursor: 1, ProjectionVersion: 1, Payload: append([]byte(nil), prepared.EventPayload...)}).Error; err != nil {
			return err
		}
		if err := tx.Create(&projectionCommitRecord{TenantID: prepared.Scope.TenantID, OwnerUserID: prepared.Scope.OwnerUserID, RunID: prepared.Scope.RunID, CommitID: prepared.CommitID, Fingerprint: fingerprint, Version: 1, SnapshotJSON: snapshotJSON}).Error; err != nil {
			return err
		}
		result = prepared.Snapshot
		return nil
	})
	if err != nil {
		if recovered, found, recoverErr := r.recoverProjectionCommit(ctx, prepared.Scope, prepared.CommitID, fingerprint); recoverErr != nil {
			return imageagent.RunProjection{}, recoverErr
		} else if found {
			return recovered, nil
		}
	}
	return result, err
}

func (r *gormRepository) GetProjection(ctx context.Context, scope imageagent.RunScope) (imageagent.RunProjection, error) {
	if err := validateScope(scope); err != nil {
		return imageagent.RunProjection{}, err
	}
	if _, err := r.findRun(ctx, r.db, scope); err != nil {
		return imageagent.RunProjection{}, err
	}
	var row projectionRecord
	err := scopedWhere(r.db.WithContext(ctx), scope).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return imageagent.RunProjection{}, imageagent.ErrProjectionSnapshotMissing
	}
	if err != nil {
		return imageagent.RunProjection{}, fmt.Errorf("get image agent projection: %w", err)
	}
	var result imageagent.RunProjection
	if err := decodeProjection(row.SnapshotJSON, &result); err != nil {
		return imageagent.RunProjection{}, err
	}
	if err := imageagent.ValidateProjectionSnapshot(scope, result); err != nil {
		return imageagent.RunProjection{}, err
	}
	return result, nil
}

func (r *gormRepository) CommitProjection(ctx context.Context, input imageagent.ProjectionCommit) (imageagent.RunProjection, error) {
	if err := validateScope(input.Scope); err != nil {
		return imageagent.RunProjection{}, err
	}
	fingerprint, err := projectionCommitFingerprint(input)
	if err != nil {
		return imageagent.RunProjection{}, err
	}
	if strings.TrimSpace(input.CommitID) == "" || strings.TrimSpace(input.EventType) == "" {
		return imageagent.RunProjection{}, fmt.Errorf("projection commit ID and event type are required")
	}
	var result imageagent.RunProjection
	err = withProjectionTransaction(ctx, r.db, func(tx *gorm.DB) error {
		if existing, found, err := findProjectionCommit(ctx, tx, input.Scope, input.CommitID); err != nil {
			return err
		} else if found {
			if existing.Fingerprint != fingerprint {
				return imageagent.ErrRevisionConflict
			}
			return decodeProjection(existing.SnapshotJSON, &result)
		}
		lockedRun, err := r.findRunForUpdate(ctx, tx, input.Scope)
		if err != nil {
			return err
		}
		var current projectionRecord
		err = scopedWhere(tx.Clauses(clause.Locking{Strength: "UPDATE"}), input.Scope).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return imageagent.ErrProjectionSnapshotMissing
		}
		if err != nil {
			return err
		}
		if existing, found, err := findProjectionCommit(ctx, tx, input.Scope, input.CommitID); err != nil {
			return err
		} else if found {
			if existing.Fingerprint != fingerprint {
				return imageagent.ErrRevisionConflict
			}
			return decodeProjection(existing.SnapshotJSON, &result)
		}
		if current.Version != input.ExpectedProjectionVersion {
			return imageagent.ErrRevisionConflict
		}
		var currentSnapshot imageagent.RunProjection
		if err := decodeProjection(current.SnapshotJSON, &currentSnapshot); err != nil {
			return err
		}
		persistedRun, err := recordToRun(lockedRun)
		if err != nil {
			return err
		}
		if err := validateProjectionPreconditions(currentSnapshot, persistedRun, input); err != nil {
			return err
		}
		if err := validateProjectionMutation(currentSnapshot, input); err != nil {
			return err
		}
		if err := r.applyGormProjectionMutation(ctx, tx, input); err != nil {
			return err
		}
		next := input.Snapshot
		next.LastEventID = current.Version + 1
		next.ProjectionVersion = next.LastEventID
		next.Actions = imageagent.AllowedActions(next.Run)
		if err := imageagent.ValidateProjectionSnapshot(input.Scope, next); err != nil {
			return err
		}
		snapshotJSON, err := json.Marshal(next)
		if err != nil {
			return err
		}
		updated := scopedWhere(tx.Model(&projectionRecord{}), input.Scope).Where("version = ?", current.Version).Updates(map[string]any{"version": next.ProjectionVersion, "snapshot_json": snapshotJSON})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		if err := tx.Create(&eventRecord{TenantID: input.Scope.TenantID, OwnerUserID: input.Scope.OwnerUserID, RunID: input.Scope.RunID, Type: input.EventType, Cursor: next.ProjectionVersion, ProjectionVersion: next.ProjectionVersion, Payload: append([]byte(nil), input.EventPayload...)}).Error; err != nil {
			return err
		}
		if err := tx.Create(&projectionCommitRecord{TenantID: input.Scope.TenantID, OwnerUserID: input.Scope.OwnerUserID, RunID: input.Scope.RunID, CommitID: input.CommitID, Fingerprint: fingerprint, Version: next.ProjectionVersion, SnapshotJSON: snapshotJSON}).Error; err != nil {
			return err
		}
		result = next
		return nil
	})
	if err != nil {
		if recovered, found, recoverErr := r.recoverProjectionCommit(ctx, input.Scope, input.CommitID, fingerprint); recoverErr != nil {
			return imageagent.RunProjection{}, recoverErr
		} else if found {
			return recovered, nil
		}
	}
	return result, err
}

func (r *gormRepository) recoverProjectionCommit(ctx context.Context, scope imageagent.RunScope, commitID, fingerprint string) (imageagent.RunProjection, bool, error) {
	for attempt := 0; attempt < 8; attempt++ {
		existing, found, err := findProjectionCommit(ctx, r.db, scope, commitID)
		if err == nil && found {
			if existing.Fingerprint != fingerprint {
				return imageagent.RunProjection{}, false, imageagent.ErrRevisionConflict
			}
			var projection imageagent.RunProjection
			if err := decodeProjection(existing.SnapshotJSON, &projection); err != nil {
				return imageagent.RunProjection{}, false, err
			}
			return projection, true, nil
		}
		if err != nil && !isConcurrentProjectionError(err) {
			return imageagent.RunProjection{}, false, err
		}
		if attempt < 7 {
			time.Sleep(time.Duration(attempt+1) * time.Millisecond)
		}
	}
	return imageagent.RunProjection{}, false, nil
}

func isConcurrentProjectionError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "locked") || strings.Contains(message, "busy") || strings.Contains(message, "serialization")
}

func withProjectionTransaction(ctx context.Context, db *gorm.DB, transaction func(*gorm.DB) error) error {
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		err = db.WithContext(ctx).Transaction(transaction)
		if err == nil || !isConcurrentProjectionError(err) {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * time.Millisecond)
	}
	return err
}

func validateProjectionMutation(current imageagent.RunProjection, input imageagent.ProjectionCommit) error {
	if err := validateSlotProjectionMutationIdentity(current, input); err != nil {
		return err
	}
	expectedRun := current.Run
	if input.RunMutation != nil {
		expectedRun.Status = input.RunMutation.Status
		expectedRun.CurrentNode = input.RunMutation.CurrentNode
		expectedRun.ActivePlanRevision = input.RunMutation.ActivePlanRevision
		expectedRun.Block = cloneBlock(input.RunMutation.Block)
		expectedRun.Version++
	}
	expectedPlan := current.Plan
	expectedSlots := append([]imageagent.SlotProjection(nil), current.Slots...)
	if input.PlanMutation != nil {
		expectedPlan = clonePlan(input.PlanMutation.Plan)
		expectedRun.ActivePlanRevision = expectedPlan.Revision
		expectedSlots = initialSlotProjections(expectedPlan)
	}
	if input.SlotMutation != nil {
		found := false
		for index := range expectedSlots {
			if expectedSlots[index].Slot.ID == input.SlotMutation.Result.SlotID {
				expectedSlots[index] = input.SlotMutation.Projection
				found = true
				break
			}
		}
		if !found {
			return imageagent.ErrRevisionConflict
		}
	}
	if !reflect.DeepEqual(input.Snapshot.Run, expectedRun) {
		return fmt.Errorf("%w: run snapshot does not match normalized mutation", imageagent.ErrRevisionConflict)
	}
	if !reflect.DeepEqual(input.Snapshot.Plan, expectedPlan) {
		return fmt.Errorf("%w: plan snapshot does not match normalized mutation", imageagent.ErrRevisionConflict)
	}
	if !reflect.DeepEqual(input.Snapshot.Slots, expectedSlots) {
		return fmt.Errorf("%w: slot snapshot does not match normalized mutation", imageagent.ErrRevisionConflict)
	}
	if !reflect.DeepEqual(input.Snapshot.AssetCatalog, current.AssetCatalog) {
		return fmt.Errorf("%w: catalog snapshot is immutable", imageagent.ErrRevisionConflict)
	}
	if err := imageagent.ValidatePlanAgainstCatalog(input.Snapshot.Plan, current.AssetCatalog); err != nil {
		return fmt.Errorf("%w: projection plan is outside the persisted catalog: %v", imageagent.ErrRevisionConflict, err)
	}
	return nil
}

func validateSlotProjectionMutationIdentity(current imageagent.RunProjection, input imageagent.ProjectionCommit) error {
	mutation := input.SlotMutation
	if mutation == nil {
		return nil
	}
	if mutation.PlanRevision != current.Run.ActivePlanRevision || mutation.PlanRevision != current.Plan.Revision ||
		mutation.Attempt.PlanRevision != mutation.PlanRevision ||
		mutation.Attempt.TenantID != input.Scope.TenantID || mutation.Attempt.OwnerUserID != input.Scope.OwnerUserID || mutation.Attempt.RunID != input.Scope.RunID ||
		mutation.Result.SlotID == "" || mutation.Result.SlotID != mutation.Attempt.SlotID || mutation.Result.SlotID != mutation.Projection.Slot.ID ||
		mutation.Result.Attempt <= 0 || mutation.Result.Attempt != mutation.Attempt.Attempt || mutation.Result.Attempt != mutation.Projection.Attempt {
		return fmt.Errorf("%w: slot mutation identity does not match the active run", imageagent.ErrRevisionConflict)
	}
	var currentSlot *imageagent.SlotProjection
	for index := range current.Slots {
		if current.Slots[index].Slot.ID == mutation.Result.SlotID {
			currentSlot = &current.Slots[index]
			break
		}
	}
	if currentSlot == nil || mutation.Result.Attempt != currentSlot.Attempt+1 {
		return fmt.Errorf("%w: slot mutation attempt does not follow the current slot", imageagent.ErrRevisionConflict)
	}
	expectedSlot := cloneSlot(currentSlot.Slot)
	expectedSlot.Status = mutation.Result.Status
	if !reflect.DeepEqual(mutation.Projection.Slot, expectedSlot) || mutation.Projection.ErrorCode != mutation.Result.ErrorCode {
		return fmt.Errorf("%w: slot mutation projection does not match the current slot", imageagent.ErrRevisionConflict)
	}
	candidateIDs := make([]string, 0, len(mutation.Projection.Candidates))
	for _, candidate := range mutation.Projection.Candidates {
		candidateIDs = append(candidateIDs, candidate.AssetID)
	}
	if !slices.Equal(candidateIDs, mutation.Result.CandidateAssetIDs) {
		return fmt.Errorf("%w: slot mutation candidate identity does not match", imageagent.ErrRevisionConflict)
	}
	return nil
}

func validateProjectionPreconditions(current imageagent.RunProjection, persistedRun imageagent.Run, input imageagent.ProjectionCommit) error {
	if current.Run.Version != persistedRun.Version || current.Run.ActivePlanRevision != persistedRun.ActivePlanRevision {
		return fmt.Errorf("%w: normalized run and public projection disagree", imageagent.ErrRevisionConflict)
	}
	if input.RunMutation != nil && persistedRun.Version != input.ExpectedRunVersion {
		return imageagent.ErrRevisionConflict
	}
	if input.PlanMutation != nil {
		if persistedRun.ActivePlanRevision != input.PlanMutation.ExpectedActiveRevision {
			return imageagent.ErrRevisionConflict
		}
		if err := imageagent.ValidatePlan(input.PlanMutation.Plan); err != nil {
			return err
		}
		if err := imageagent.ValidatePlanAgainstCatalog(input.PlanMutation.Plan, current.AssetCatalog); err != nil {
			return fmt.Errorf("%w: replacement plan is outside the persisted catalog: %v", imageagent.ErrRevisionConflict, err)
		}
	}
	return nil
}

func (r *gormRepository) applyGormProjectionMutation(ctx context.Context, tx *gorm.DB, input imageagent.ProjectionCommit) error {
	if input.RunMutation != nil {
		blockJSON, err := marshalJSON(input.RunMutation.Block)
		if err != nil {
			return err
		}
		updates := scopedRunWhere(tx.Model(&runRecord{}), input.Scope).Where("version = ?", input.ExpectedRunVersion).Updates(map[string]any{"status": string(input.RunMutation.Status), "current_node": input.RunMutation.CurrentNode, "active_plan_revision": input.RunMutation.ActivePlanRevision, "block_json": blockJSON, "version": input.ExpectedRunVersion + 1})
		if updates.Error != nil {
			return updates.Error
		}
		if updates.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
	}
	if input.PlanMutation != nil {
		mutation := input.PlanMutation
		updated := scopedRunWhere(tx.Model(&runRecord{}), input.Scope).Update("active_plan_revision", mutation.Plan.Revision)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		planRow, slots, err := planToRecords(input.Scope, mutation.Plan)
		if err != nil {
			return err
		}
		if err := tx.Create(&planRow).Error; err != nil {
			return err
		}
		if len(slots) > 0 {
			if err := tx.Create(&slots).Error; err != nil {
				return err
			}
		}
	}
	if input.SlotMutation != nil {
		mutation := input.SlotMutation
		candidateIDs := make([]string, 0, len(mutation.Projection.Candidates))
		for _, candidate := range mutation.Projection.Candidates {
			candidateIDs = append(candidateIDs, candidate.AssetID)
		}
		candidates, _ := marshalJSON(candidateIDs)
		updated := scopedWhere(tx.Model(&slotRecord{}), input.Scope).Where("plan_revision = ? AND id = ?", mutation.PlanRevision, mutation.Result.SlotID).Updates(map[string]any{"attempt": mutation.Result.Attempt, "status": string(mutation.Result.Status), "candidate_asset_ids": candidates, "error_code": mutation.Result.ErrorCode})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return imageagent.ErrRevisionConflict
		}
		row := attemptRecord{TenantID: mutation.Attempt.TenantID, OwnerUserID: mutation.Attempt.OwnerUserID, RunID: mutation.Attempt.RunID, PlanRevision: mutation.Attempt.PlanRevision, SlotID: mutation.Attempt.SlotID, Attempt: mutation.Attempt.Attempt, Node: mutation.Attempt.Node, IdempotencyKey: mutation.Attempt.IdempotencyKey, Outcome: mutation.Attempt.Outcome, ErrorCategory: mutation.Attempt.ErrorCategory}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func prepareInitialization(input imageagent.ProjectionInitialization) (imageagent.ProjectionInitialization, string, error) {
	if err := validateScope(input.Scope); err != nil {
		return input, "", err
	}
	if input.Scope != imageagent.ScopeForRun(input.Run) {
		return input, "", imageagent.ErrRunNotFound
	}
	if strings.TrimSpace(input.CommitID) == "" || strings.TrimSpace(input.EventType) == "" {
		return input, "", fmt.Errorf("projection initialization commit ID and event type are required")
	}
	if err := validateRun(&input.Run); err != nil {
		return input, "", err
	}
	input.Run.MaxConcurrentSlots = imageagent.NormalizeMaxConcurrentSlots(input.Run.MaxConcurrentSlots)
	if err := imageagent.ValidatePlan(input.Plan); err != nil {
		return input, "", err
	}
	catalog, err := imageagent.NormalizeAssetCatalog(input.Catalog)
	if err != nil {
		return input, "", err
	}
	input.Catalog = catalog
	if err := imageagent.ValidatePlanAgainstCatalog(input.Plan, catalog); err != nil {
		return input, "", err
	}
	input.Run.ActivePlanRevision = input.Plan.Revision
	input.Snapshot.Run = input.Run
	input.Snapshot.Plan = input.Plan
	input.Snapshot.AssetCatalog = catalog
	input.Snapshot.LastEventID = 1
	input.Snapshot.ProjectionVersion = 1
	input.Snapshot.Actions = imageagent.AllowedActions(input.Run)
	if len(input.Snapshot.Slots) == 0 {
		input.Snapshot.Slots = initialSlotProjections(input.Plan)
	}
	if err := imageagent.ValidateProjectionSnapshot(input.Scope, input.Snapshot); err != nil {
		return input, "", err
	}
	fingerprint, err := initializationFingerprint(input)
	return input, fingerprint, err
}

func materializeRepositoryCatalogTimestamp(input imageagent.ProjectionInitialization, createdAt time.Time) imageagent.ProjectionInitialization {
	if !input.Catalog.Manifest.CreatedAt.IsZero() {
		return input
	}
	input.Catalog.Manifest.CreatedAt = createdAt
	input.Snapshot.AssetCatalog.Manifest.CreatedAt = createdAt
	return input
}

func initialSlotProjections(plan imageagent.Plan) []imageagent.SlotProjection {
	result := make([]imageagent.SlotProjection, len(plan.Slots))
	for i, slot := range plan.Slots {
		result[i] = imageagent.SlotProjection{Slot: slot}
	}
	return result
}
func initializationFingerprint(input imageagent.ProjectionInitialization) (string, error) {
	return hashJSON(struct {
		Scope               imageagent.RunScope
		Run                 imageagent.Run
		Plan                imageagent.Plan
		Catalog             imageagent.AssetCatalog
		Snapshot            imageagent.RunProjection
		CommitID, EventType string
		Payload             json.RawMessage
	}{input.Scope, input.Run, input.Plan, input.Catalog, input.Snapshot, input.CommitID, input.EventType, input.EventPayload})
}
func projectionCommitFingerprint(input imageagent.ProjectionCommit) (string, error) {
	return hashJSON(input)
}
func hashJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
func cloneProjection(value imageagent.RunProjection) imageagent.RunProjection {
	encoded, _ := json.Marshal(value)
	var cloned imageagent.RunProjection
	_ = json.Unmarshal(encoded, &cloned)
	cloned.Run.MaxConcurrentSlots = imageagent.NormalizeMaxConcurrentSlots(cloned.Run.MaxConcurrentSlots)
	return cloned
}
func decodeProjection(raw []byte, target *imageagent.RunProjection) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode image agent projection: %w", err)
	}
	target.Run.MaxConcurrentSlots = imageagent.NormalizeMaxConcurrentSlots(target.Run.MaxConcurrentSlots)
	return nil
}
func findProjectionCommit(ctx context.Context, db *gorm.DB, scope imageagent.RunScope, commitID string) (projectionCommitRecord, bool, error) {
	var row projectionCommitRecord
	err := scopedWhere(db.WithContext(ctx), scope).Where("commit_id = ?", commitID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return row, false, nil
	}
	if err != nil {
		return row, false, err
	}
	return row, true, nil
}
func scopedWhere(db *gorm.DB, scope imageagent.RunScope) *gorm.DB {
	return db.Where("tenant_id = ? AND owner_user_id = ? AND run_id = ?", scope.TenantID, scope.OwnerUserID, scope.RunID)
}
func scopedRunWhere(db *gorm.DB, scope imageagent.RunScope) *gorm.DB {
	return db.Where("tenant_id = ? AND owner_user_id = ? AND id = ?", scope.TenantID, scope.OwnerUserID, scope.RunID)
}
func mapCreateConflict(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", imageagent.ErrRevisionConflict, err)
}
