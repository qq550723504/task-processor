package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"task-processor/internal/imageagent"
)

type memoryRepository struct {
	mu                sync.Mutex
	runs              map[string]imageagent.Run
	plans             map[string]map[int64]imageagent.Plan
	slots             map[string]slotResultRecord
	attempts          map[string]imageagent.StepAttempt
	byNumber          map[string]imageagent.StepAttempt
	events            map[string][]imageagent.RunEvent
	catalogs          map[string]imageagent.AssetCatalog
	projections       map[string]imageagent.RunProjection
	projectionCommits map[string]map[string]projectionCommitMemory
	slotEffects       map[string]imageagent.SlotExternalEffectAttempt
	slotEffectsV3     map[string]imageagent.SlotEffectV3Attempt
	reservedUsage     map[string]imageagent.UsageVector
	clock             func() time.Time
}

type projectionCommitMemory struct {
	fingerprint string
	version     int64
	snapshot    imageagent.RunProjection
}

type slotResultRecord struct {
	slot   imageagent.Slot
	result imageagent.SlotResult
}

func NewMemoryRepository() imageagent.Repository {
	return newMemoryRepositoryWithClock(time.Now)
}

// NewMemoryRepositoryWithClock provides deterministic database-time semantics
// for lease and fencing verification while retaining the production repository
// contract.
func NewMemoryRepositoryWithClock(clock func() time.Time) imageagent.Repository {
	return newMemoryRepositoryWithClock(clock)
}

func newMemoryRepositoryWithClock(clock func() time.Time) imageagent.Repository {
	if clock == nil {
		clock = time.Now
	}
	return &memoryRepository{
		runs:              map[string]imageagent.Run{},
		plans:             map[string]map[int64]imageagent.Plan{},
		slots:             map[string]slotResultRecord{},
		attempts:          map[string]imageagent.StepAttempt{},
		byNumber:          map[string]imageagent.StepAttempt{},
		events:            map[string][]imageagent.RunEvent{},
		catalogs:          map[string]imageagent.AssetCatalog{},
		projections:       map[string]imageagent.RunProjection{},
		projectionCommits: map[string]map[string]projectionCommitMemory{},
		slotEffects:       map[string]imageagent.SlotExternalEffectAttempt{},
		slotEffectsV3:     map[string]imageagent.SlotEffectV3Attempt{},
		reservedUsage:     map[string]imageagent.UsageVector{},
		clock:             clock,
	}
}

func (r *memoryRepository) CreateRun(_ context.Context, run *imageagent.Run) error {
	if err := validateRun(run); err != nil {
		return err
	}
	prepared := cloneRun(*run)
	prepared.MaxConcurrentSlots = imageagent.NormalizeMaxConcurrentSlots(prepared.MaxConcurrentSlots)
	r.mu.Lock()
	defer r.mu.Unlock()
	key := runKey(prepared.TenantID, prepared.UserID, prepared.ID)
	if existing, exists := r.runs[key]; exists {
		if sameRun(existing, prepared) {
			return nil
		}
		return imageagent.ErrRevisionConflict
	}
	for _, existing := range r.runs {
		if existing.TenantID == prepared.TenantID && existing.UserID == prepared.UserID && existing.IdempotencyKey == prepared.IdempotencyKey {
			return imageagent.ErrRevisionConflict
		}
	}
	r.runs[key] = prepared
	return nil
}

func (r *memoryRepository) GetRun(_ context.Context, scope imageagent.RunScope) (*imageagent.Run, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	run, exists := r.runs[scopeKey(scope)]
	if !exists {
		return nil, imageagent.ErrRunNotFound
	}
	cloned := cloneRun(run)
	return &cloned, nil
}

func (r *memoryRepository) UpdateRun(_ context.Context, scope imageagent.RunScope, expectedVersion int64, mutation imageagent.RunMutation) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(scope)
	run, exists := r.runs[key]
	if !exists {
		return imageagent.ErrRunNotFound
	}
	if run.Version != expectedVersion {
		return imageagent.ErrRevisionConflict
	}
	payload, err := json.Marshal(mutation)
	if err != nil {
		return fmt.Errorf("marshal run mutation event: %w", err)
	}
	run.Status = mutation.Status
	run.CurrentNode = mutation.CurrentNode
	run.ActivePlanRevision = mutation.ActivePlanRevision
	run.Block = cloneBlock(mutation.Block)
	run.Version++
	r.runs[key] = run

	cursor := nextCursor(r.events[key])
	r.events[key] = append(r.events[key], imageagent.RunEvent{
		TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, Type: "run.updated", Cursor: cursor,
		ProjectionVersion: cursor, Payload: append(json.RawMessage(nil), payload...),
	})
	return nil
}

func (r *memoryRepository) AppendPlan(_ context.Context, scope imageagent.RunScope, expectedActiveRevision int64, plan imageagent.Plan) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if err := imageagent.ValidatePlan(plan); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(scope)
	run, exists := r.runs[key]
	if !exists {
		return imageagent.ErrRunNotFound
	}
	if r.plans[key] == nil {
		r.plans[key] = map[int64]imageagent.Plan{}
	}
	for _, existing := range r.plans[key] {
		if existing.Revision == plan.Revision || existing.IdempotencyKey == plan.IdempotencyKey {
			if samePlan(existing, plan) {
				return nil
			}
			return imageagent.ErrRevisionConflict
		}
	}
	if run.ActivePlanRevision != expectedActiveRevision {
		return imageagent.ErrRevisionConflict
	}
	r.plans[key][plan.Revision] = clonePlan(plan)
	for _, slot := range plan.Slots {
		r.slots[slotKey(scope, plan.Revision, slot.ID)] = slotResultRecord{slot: cloneSlot(slot)}
	}
	run.ActivePlanRevision = plan.Revision
	r.runs[key] = run
	return nil
}

func (r *memoryRepository) SaveSlotResult(_ context.Context, scope imageagent.RunScope, expectedActiveRevision int64, result imageagent.SlotResult) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	if strings.TrimSpace(result.SlotID) == "" {
		return fmt.Errorf("slot ID cannot be empty")
	}
	if result.Attempt <= 0 {
		return fmt.Errorf("slot result attempt must be positive")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(scope)
	run, exists := r.runs[key]
	if !exists {
		return imageagent.ErrRunNotFound
	}
	if run.ActivePlanRevision != expectedActiveRevision {
		return imageagent.ErrRevisionConflict
	}
	slotKey := slotKey(scope, expectedActiveRevision, result.SlotID)
	stored, exists := r.slots[slotKey]
	if !exists {
		return imageagent.ErrRevisionConflict
	}
	if stored.result.Attempt == result.Attempt {
		if sameSlotResult(stored.result, result) {
			return nil
		}
		return imageagent.ErrRevisionConflict
	}
	if stored.result.Attempt > result.Attempt {
		return imageagent.ErrRevisionConflict
	}
	stored.result = cloneSlotResult(result)
	stored.slot.Status = result.Status
	r.slots[slotKey] = stored
	return nil
}

func (r *memoryRepository) AppendAttempt(_ context.Context, attempt imageagent.StepAttempt) error {
	if err := validateAttempt(attempt); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runs[runKey(attempt.TenantID, attempt.OwnerUserID, attempt.RunID)]; !exists {
		return imageagent.ErrRunNotFound
	}
	key := attemptKey(attempt)
	if existing, exists := r.attempts[key]; exists {
		if sameStepAttempt(existing, attempt) {
			return nil
		}
		return imageagent.ErrRevisionConflict
	}
	numberKey := attemptNumberKey(attempt)
	if existing, exists := r.byNumber[numberKey]; exists {
		if sameStepAttempt(existing, attempt) {
			return nil
		}
		return imageagent.ErrRevisionConflict
	}
	r.attempts[key] = attempt
	r.byNumber[numberKey] = attempt
	return nil
}

func (r *memoryRepository) AppendEvent(_ context.Context, event imageagent.RunEvent) error {
	if err := validateEvent(event); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := runKey(event.TenantID, event.OwnerUserID, event.RunID)
	if _, exists := r.runs[key]; !exists {
		return imageagent.ErrRunNotFound
	}
	if event.Cursor < nextCursor(r.events[key]) {
		return imageagent.ErrRevisionConflict
	}
	event.Payload = append(json.RawMessage(nil), event.Payload...)
	r.events[key] = append(r.events[key], event)
	return nil
}

func (r *memoryRepository) AppendProjectionEvent(_ context.Context, event imageagent.RunEvent) (imageagent.RunEvent, error) {
	if strings.TrimSpace(event.TenantID) == "" || strings.TrimSpace(event.OwnerUserID) == "" || strings.TrimSpace(event.RunID) == "" || strings.TrimSpace(event.Type) == "" {
		return imageagent.RunEvent{}, fmt.Errorf("event tenant, owner, run, and type are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := runKey(event.TenantID, event.OwnerUserID, event.RunID)
	if _, exists := r.runs[key]; !exists {
		return imageagent.RunEvent{}, imageagent.ErrRunNotFound
	}
	event.Cursor = nextCursor(r.events[key])
	event.ProjectionVersion = event.Cursor
	event.Payload = append(json.RawMessage(nil), event.Payload...)
	r.events[key] = append(r.events[key], event)
	return event, nil
}

func (r *memoryRepository) SaveAssetCatalog(_ context.Context, scope imageagent.RunScope, catalog imageagent.AssetCatalog) error {
	if err := validateScope(scope); err != nil {
		return err
	}
	normalized, err := imageagent.NormalizeAssetCatalog(catalog)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(scope)
	if _, exists := r.runs[key]; !exists {
		return imageagent.ErrRunNotFound
	}
	cloned := cloneCatalog(normalized)
	if existing, exists := r.catalogs[key]; exists {
		if existing.Manifest.Version != cloned.Manifest.Version || existing.Manifest.Hash != cloned.Manifest.Hash || !reflect.DeepEqual(existing.Assets, cloned.Assets) || !reflect.DeepEqual(existing.ProductContext, cloned.ProductContext) {
			return imageagent.ErrRevisionConflict
		}
		return nil
	}
	r.catalogs[key] = cloned
	return nil
}

func (r *memoryRepository) GetAssetCatalog(_ context.Context, scope imageagent.RunScope) (imageagent.AssetCatalog, error) {
	if err := validateScope(scope); err != nil {
		return imageagent.AssetCatalog{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	catalog, exists := r.catalogs[scopeKey(scope)]
	if !exists {
		if _, runExists := r.runs[scopeKey(scope)]; !runExists {
			return imageagent.AssetCatalog{}, imageagent.ErrRunNotFound
		}
		return imageagent.AssetCatalog{}, imageagent.ErrCatalogSnapshotMissing
	}
	return cloneCatalog(catalog), nil
}

func (r *memoryRepository) ListEvents(_ context.Context, scope imageagent.RunScope, afterCursor int64, limit int) ([]imageagent.RunEvent, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := scopeKey(scope)
	if _, exists := r.runs[key]; !exists {
		return nil, imageagent.ErrRunNotFound
	}
	if limit <= 0 {
		return []imageagent.RunEvent{}, nil
	}
	events := append([]imageagent.RunEvent(nil), r.events[key]...)
	sort.Slice(events, func(i, j int) bool { return events[i].Cursor < events[j].Cursor })
	result := make([]imageagent.RunEvent, 0, len(events))
	for _, event := range events {
		if event.Cursor <= afterCursor {
			continue
		}
		event.Payload = append(json.RawMessage(nil), event.Payload...)
		result = append(result, event)
		if len(result) >= limit {
			break
		}
	}
	return result, nil
}

func validateRun(run *imageagent.Run) error {
	if run == nil || strings.TrimSpace(run.ID) == "" || strings.TrimSpace(run.TenantID) == "" || strings.TrimSpace(run.UserID) == "" || strings.TrimSpace(run.IdempotencyKey) == "" {
		return fmt.Errorf("run ID, tenant ID, owner user ID, and idempotency key are required")
	}
	if run.Mode != imageagent.RunModeManual {
		return fmt.Errorf("image agent run mode must be manual")
	}
	return nil
}

func validateScope(scope imageagent.RunScope) error {
	if strings.TrimSpace(scope.TenantID) == "" || strings.TrimSpace(scope.OwnerUserID) == "" || strings.TrimSpace(scope.RunID) == "" {
		return imageagent.ErrRunNotFound
	}
	return nil
}

func validateAttempt(attempt imageagent.StepAttempt) error {
	if strings.TrimSpace(attempt.TenantID) == "" || strings.TrimSpace(attempt.OwnerUserID) == "" || strings.TrimSpace(attempt.RunID) == "" || strings.TrimSpace(attempt.SlotID) == "" || strings.TrimSpace(attempt.Node) == "" || strings.TrimSpace(attempt.IdempotencyKey) == "" || attempt.PlanRevision <= 0 || attempt.Attempt <= 0 {
		return fmt.Errorf("attempt tenant, owner, run, slot, positive plan revision, node, idempotency key, and positive attempt are required")
	}
	return nil
}

func validateEvent(event imageagent.RunEvent) error {
	if strings.TrimSpace(event.TenantID) == "" || strings.TrimSpace(event.OwnerUserID) == "" || strings.TrimSpace(event.RunID) == "" || strings.TrimSpace(event.Type) == "" || event.Cursor <= 0 {
		return fmt.Errorf("event tenant, owner, run, type, and positive cursor are required")
	}
	return nil
}

func runKey(tenantID, ownerUserID, runID string) string {
	return tenantID + "\x00" + ownerUserID + "\x00" + runID
}

func scopeKey(scope imageagent.RunScope) string {
	return runKey(scope.TenantID, scope.OwnerUserID, scope.RunID)
}

func slotKey(scope imageagent.RunScope, revision int64, slotID string) string {
	return fmt.Sprintf("%s\x00%d\x00%s", scopeKey(scope), revision, slotID)
}

func attemptKey(attempt imageagent.StepAttempt) string {
	return fmt.Sprintf("%s\x00%d\x00%s\x00%s", runKey(attempt.TenantID, attempt.OwnerUserID, attempt.RunID), attempt.PlanRevision, attempt.SlotID, attempt.IdempotencyKey)
}

func attemptNumberKey(attempt imageagent.StepAttempt) string {
	return fmt.Sprintf("%s\x00%d\x00%s\x00%d", runKey(attempt.TenantID, attempt.OwnerUserID, attempt.RunID), attempt.PlanRevision, attempt.SlotID, attempt.Attempt)
}

func nextCursor(events []imageagent.RunEvent) int64 {
	var cursor int64
	for _, event := range events {
		if event.Cursor > cursor {
			cursor = event.Cursor
		}
	}
	return cursor + 1
}

func cloneRun(run imageagent.Run) imageagent.Run {
	run.Block = cloneBlock(run.Block)
	return run
}

func clonePlan(plan imageagent.Plan) imageagent.Plan {
	plan.SourceAssetIDs = append([]string(nil), plan.SourceAssetIDs...)
	plan.StyleReferenceIDs = append([]string(nil), plan.StyleReferenceIDs...)
	slots := make([]imageagent.Slot, len(plan.Slots))
	for i, slot := range plan.Slots {
		slots[i] = cloneSlot(slot)
	}
	plan.Slots = slots
	return plan
}

func cloneSlot(slot imageagent.Slot) imageagent.Slot {
	slot.SourceAssetIDs = append([]string(nil), slot.SourceAssetIDs...)
	slot.StyleReferenceIDs = append([]string(nil), slot.StyleReferenceIDs...)
	return slot
}

func cloneSlotResult(result imageagent.SlotResult) imageagent.SlotResult {
	result.CandidateAssetIDs = append([]string(nil), result.CandidateAssetIDs...)
	return result
}

func cloneBlock(block *imageagent.Block) *imageagent.Block {
	if block == nil {
		return nil
	}
	cloned := *block
	return &cloned
}

func cloneCatalog(catalog imageagent.AssetCatalog) imageagent.AssetCatalog {
	assets := make([]imageagent.AuthorizedAsset, len(catalog.Assets))
	for index, asset := range catalog.Assets {
		asset.Metadata = cloneMetadata(asset.Metadata)
		assets[index] = asset
	}
	catalog.Assets = assets
	catalog.ProductContext.Attributes = cloneMetadata(catalog.ProductContext.Attributes)
	return catalog
}

func cloneMetadata(value map[string]string) map[string]string {
	if value == nil {
		return nil
	}
	cloned := make(map[string]string, len(value))
	for key, item := range value {
		cloned[key] = item
	}
	return cloned
}

func sameRun(left, right imageagent.Run) bool {
	if left.StartedAt.IsZero() || right.StartedAt.IsZero() {
		left.StartedAt, right.StartedAt = time.Time{}, time.Time{}
	}
	return reflect.DeepEqual(left, right)
}

func samePlan(left, right imageagent.Plan) bool { return reflect.DeepEqual(left, right) }

func sameSlotResult(left, right imageagent.SlotResult) bool { return reflect.DeepEqual(left, right) }

func sameStepAttempt(left, right imageagent.StepAttempt) bool { return reflect.DeepEqual(left, right) }
