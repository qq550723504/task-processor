package store

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"task-processor/internal/imageagent"
)

type memoryRepository struct {
	mu       sync.Mutex
	runs     map[string]imageagent.Run
	plans    map[string]map[int64]imageagent.Plan
	slots    map[string]slotResultRecord
	attempts map[string]imageagent.StepAttempt
	byNumber map[string]imageagent.StepAttempt
	events   map[string][]imageagent.RunEvent
}

type slotResultRecord struct {
	slot   imageagent.Slot
	result imageagent.SlotResult
}

func NewMemoryRepository() imageagent.Repository {
	return &memoryRepository{
		runs:     map[string]imageagent.Run{},
		plans:    map[string]map[int64]imageagent.Plan{},
		slots:    map[string]slotResultRecord{},
		attempts: map[string]imageagent.StepAttempt{},
		byNumber: map[string]imageagent.StepAttempt{},
		events:   map[string][]imageagent.RunEvent{},
	}
}

func (r *memoryRepository) CreateRun(_ context.Context, run *imageagent.Run) error {
	if err := validateRun(run); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := runKey(run.TenantID, run.ID)
	if existing, exists := r.runs[key]; exists {
		if sameRun(existing, *run) {
			return nil
		}
		return imageagent.ErrRevisionConflict
	}
	for _, existing := range r.runs {
		if existing.TenantID == run.TenantID && existing.IdempotencyKey == run.IdempotencyKey {
			return imageagent.ErrRevisionConflict
		}
	}
	r.runs[key] = cloneRun(*run)
	return nil
}

func (r *memoryRepository) GetRun(_ context.Context, scope imageagent.RunScope) (*imageagent.Run, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	run, exists := r.runs[runKey(scope.TenantID, scope.RunID)]
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
	key := runKey(scope.TenantID, scope.RunID)
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
		TenantID: scope.TenantID, RunID: scope.RunID, Type: "run.updated", Cursor: cursor,
		ProjectionVersion: run.Version, Payload: append(json.RawMessage(nil), payload...),
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
	key := runKey(scope.TenantID, scope.RunID)
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
	key := runKey(scope.TenantID, scope.RunID)
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
	if _, exists := r.runs[runKey(attempt.TenantID, attempt.RunID)]; !exists {
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
	key := runKey(event.TenantID, event.RunID)
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

func (r *memoryRepository) ListEvents(_ context.Context, scope imageagent.RunScope, afterCursor int64, limit int) ([]imageagent.RunEvent, error) {
	if err := validateScope(scope); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := runKey(scope.TenantID, scope.RunID)
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
	if run == nil || strings.TrimSpace(run.ID) == "" || strings.TrimSpace(run.TenantID) == "" || strings.TrimSpace(run.IdempotencyKey) == "" {
		return fmt.Errorf("run ID, tenant ID, and idempotency key are required")
	}
	if run.Mode != imageagent.RunModeManual {
		return fmt.Errorf("image agent run mode must be manual")
	}
	return nil
}

func validateScope(scope imageagent.RunScope) error {
	if strings.TrimSpace(scope.TenantID) == "" || strings.TrimSpace(scope.RunID) == "" {
		return imageagent.ErrRunNotFound
	}
	return nil
}

func validateAttempt(attempt imageagent.StepAttempt) error {
	if strings.TrimSpace(attempt.TenantID) == "" || strings.TrimSpace(attempt.RunID) == "" || strings.TrimSpace(attempt.SlotID) == "" || strings.TrimSpace(attempt.Node) == "" || strings.TrimSpace(attempt.IdempotencyKey) == "" || attempt.Attempt <= 0 {
		return fmt.Errorf("attempt tenant, run, slot, node, idempotency key, and positive attempt are required")
	}
	return nil
}

func validateEvent(event imageagent.RunEvent) error {
	if strings.TrimSpace(event.TenantID) == "" || strings.TrimSpace(event.RunID) == "" || strings.TrimSpace(event.Type) == "" || event.Cursor <= 0 {
		return fmt.Errorf("event tenant, run, type, and positive cursor are required")
	}
	return nil
}

func runKey(tenantID, runID string) string { return tenantID + "\x00" + runID }

func slotKey(scope imageagent.RunScope, revision int64, slotID string) string {
	return fmt.Sprintf("%s\x00%d\x00%s", runKey(scope.TenantID, scope.RunID), revision, slotID)
}

func attemptKey(attempt imageagent.StepAttempt) string {
	return fmt.Sprintf("%s\x00%s\x00%s", runKey(attempt.TenantID, attempt.RunID), attempt.SlotID, attempt.IdempotencyKey)
}

func attemptNumberKey(attempt imageagent.StepAttempt) string {
	return fmt.Sprintf("%s\x00%s\x00%d", runKey(attempt.TenantID, attempt.RunID), attempt.SlotID, attempt.Attempt)
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

func sameRun(left, right imageagent.Run) bool { return reflect.DeepEqual(left, right) }

func samePlan(left, right imageagent.Plan) bool { return reflect.DeepEqual(left, right) }

func sameSlotResult(left, right imageagent.SlotResult) bool { return reflect.DeepEqual(left, right) }

func sameStepAttempt(left, right imageagent.StepAttempt) bool { return left == right }
