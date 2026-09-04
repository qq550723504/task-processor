package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"task-processor/internal/imageagent"
)

func (a *Activities) PersistSlotResult(ctx context.Context, input PersistSlotResultActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	result := input.Result
	if strings.TrimSpace(result.Execution.SlotID) == "" || result.Execution.Attempt <= 0 {
		return fmt.Errorf("terminal slot result requires slot ID and positive attempt")
	}
	var candidateIDs []string
	var candidates []imageagent.AssetCandidate
	for _, candidate := range result.Execution.Candidates {
		id := strings.TrimSpace(candidate.AssetID)
		if id == "" {
			if result.Status == imageagent.SlotStatusAccepted {
				return fmt.Errorf("accepted slot result contains an empty candidate asset ID")
			}
			continue
		}
		validatedURL, validateErr := imageagent.ValidateSafeImageURL(candidate.URL)
		if validateErr != nil {
			return fmt.Errorf("candidate %q has unsafe URL: %w", id, validateErr)
		}
		candidate.URL = validatedURL
		candidateIDs = append(candidateIDs, id)
		candidates = append(candidates, candidate)
	}
	if result.Status == imageagent.SlotStatusAccepted && len(candidateIDs) == 0 {
		return fmt.Errorf("accepted slot result requires at least one candidate asset ID")
	}
	outcome := "accepted"
	if result.Status != imageagent.SlotStatusAccepted {
		outcome = "blocked"
	}
	attempt := imageagent.StepAttempt{
		TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID,
		PlanRevision: input.PlanRevision, SlotID: result.Execution.SlotID, Node: "execute_slot",
		IdempotencyKey: input.AttemptKey, Attempt: result.Execution.Attempt,
		Outcome: outcome, ErrorCategory: result.ErrorCode,
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("load image agent projection: %w", err)
	}
	if slotProjectionAlreadyPersisted(current.Slots, result.Execution.SlotID, result.Execution.Attempt, result.Status, candidates, result.ErrorCode) {
		return nil
	}
	storedResult := imageagent.SlotResult{
		SlotID: result.Execution.SlotID, Attempt: result.Execution.Attempt,
		Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode,
	}
	updated := current
	found := false
	for index := range updated.Slots {
		if updated.Slots[index].Slot.ID != result.Execution.SlotID {
			continue
		}
		updated.Slots[index] = imageagent.SlotProjection{Slot: updated.Slots[index].Slot, Attempt: result.Execution.Attempt, Candidates: candidates, ErrorCode: result.ErrorCode}
		updated.Slots[index].Slot.Status = result.Status
		found = true
	}
	if !found {
		return imageagent.ErrRevisionConflict
	}
	eventPayload, err := json.Marshal(slotResultPersistedEventPayload{
		PlanRevision: input.PlanRevision, SlotID: result.Execution.SlotID,
		Attempt: result.Execution.Attempt, AttemptKey: input.AttemptKey,
		Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode,
	})
	if err != nil {
		return fmt.Errorf("encode terminal slot result event: %w", err)
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "slot:" + input.AttemptKey, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: slotResultPersistedEventType, EventPayload: eventPayload, SlotMutation: &imageagent.SlotProjectionMutation{PlanRevision: input.PlanRevision, Result: storedResult, Projection: updated.Slots[slotProjectionIndex(updated.Slots, result.Execution.SlotID)], Attempt: attempt}})
	if err != nil {
		return fmt.Errorf("commit terminal slot projection: %w", err)
	}
	return nil
}

// PersistSlotResultV3 persists only the additive v3 durable result contract.
// It is intentionally absent from RegisterActivities until Task 6 selects the
// final production wire.
func (a *Activities) PersistSlotResultV3(ctx context.Context, input PersistSlotResultV3ActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	result := input.Result
	if strings.TrimSpace(result.Published.SlotID) == "" || result.Published.Attempt <= 0 {
		return fmt.Errorf("terminal v3 slot result requires slot ID and positive attempt")
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("load image agent projection: %w", err)
	}
	slotIndex := slotProjectionIndex(current.Slots, result.Published.SlotID)
	if slotIndex < 0 {
		return imageagent.ErrRevisionConflict
	}
	if result.Status == imageagent.SlotStatusAccepted && current.Slots[slotIndex].Slot.Role == imageagent.SlotRoleMain && len(result.Published.Candidates) != 1 {
		result.Status = imageagent.SlotStatusBlocked
		result.ErrorCode = invalidMainCandidateCountCode
		result.Published.Candidates = nil
	}
	var candidateIDs []string
	var candidates []imageagent.AssetCandidate
	if result.Status == imageagent.SlotStatusAccepted {
		normalized, normalizeErr := imageagent.NormalizeSlotEffectV3PublishedResult(result.Published)
		if normalizeErr != nil {
			return fmt.Errorf("normalize terminal v3 slot result: %w", normalizeErr)
		}
		result.Published = normalized
		for _, candidate := range normalized.Candidates {
			candidateIDs = append(candidateIDs, candidate.AssetID)
			candidates = append(candidates, imageagent.AssetCandidate{
				AssetID: candidate.AssetID, SourceAssetID: candidate.SourceAssetID, DurableAsset: candidate.DurableAsset,
				Width: candidate.Width, Height: candidate.Height, Operations: append([]string(nil), candidate.Operations...),
			})
		}
	}
	outcome := "accepted"
	if result.Status != imageagent.SlotStatusAccepted {
		outcome = "blocked"
	}
	attempt := imageagent.StepAttempt{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID, PlanRevision: input.PlanRevision, SlotID: result.Published.SlotID, Node: "execute_slot_v3", IdempotencyKey: input.AttemptKey, Attempt: result.Published.Attempt, Outcome: outcome, ErrorCategory: result.ErrorCode}
	if slotProjectionAlreadyPersisted(current.Slots, result.Published.SlotID, result.Published.Attempt, result.Status, candidates, result.ErrorCode) {
		return nil
	}
	storedResult := imageagent.SlotResult{SlotID: result.Published.SlotID, Attempt: result.Published.Attempt, Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode}
	updated := current
	updated.Slots[slotIndex] = imageagent.SlotProjection{Slot: updated.Slots[slotIndex].Slot, Attempt: result.Published.Attempt, Candidates: candidates, ErrorCode: result.ErrorCode}
	updated.Slots[slotIndex].Slot.Status = result.Status
	eventPayload, err := json.Marshal(slotResultPersistedEventPayload{PlanRevision: input.PlanRevision, SlotID: result.Published.SlotID, Attempt: result.Published.Attempt, AttemptKey: input.AttemptKey, Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode})
	if err != nil {
		return fmt.Errorf("encode terminal v3 slot result event: %w", err)
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "slot-v3:" + input.AttemptKey, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: slotResultPersistedEventType, EventPayload: eventPayload, SlotMutation: &imageagent.SlotProjectionMutation{PlanRevision: input.PlanRevision, Result: storedResult, Projection: updated.Slots[slotIndex], Attempt: attempt}})
	if err != nil {
		return fmt.Errorf("commit terminal v3 slot projection: %w", err)
	}
	return nil
}

func slotProjectionAlreadyPersisted(slots []imageagent.SlotProjection, slotID string, attempt int, status imageagent.SlotStatus, candidates []imageagent.AssetCandidate, errorCode string) bool {
	for _, slot := range slots {
		if slot.Slot.ID == slotID {
			return slot.Attempt == attempt && slot.Slot.Status == status && slot.ErrorCode == errorCode && reflect.DeepEqual(slot.Candidates, candidates)
		}
	}
	return false
}

type slotResultPersistedEventPayload struct {
	PlanRevision      int64                 `json:"plan_revision"`
	SlotID            string                `json:"slot_id"`
	Attempt           int                   `json:"attempt"`
	AttemptKey        string                `json:"attempt_key"`
	Status            imageagent.SlotStatus `json:"status"`
	CandidateAssetIDs []string              `json:"candidate_asset_ids"`
	ErrorCode         string                `json:"error_code,omitempty"`
}

func (a *Activities) PersistRunState(ctx context.Context, input PersistRunStateActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("get image agent projection: %w", err)
	}
	if current.Run.ActivePlanRevision != input.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	if current.Run.Status == input.Projection.Status && current.Run.CurrentNode == input.CurrentNode &&
		reflect.DeepEqual(current.Run.Block, input.Projection.Block) && reflect.DeepEqual(current.Plan, input.Projection.Plan) &&
		reflect.DeepEqual(current.Slots, input.Projection.Slots) && current.ResultDigest == input.Projection.ResultDigest &&
		reflect.DeepEqual(current.PendingCommand, input.Projection.PendingCommand) &&
		reflect.DeepEqual(current.RecoverableEffects, input.Projection.RecoverableEffects) {
		return nil
	}
	updated := current
	updated.Run.Status = input.Projection.Status
	updated.Run.CurrentNode = input.CurrentNode
	updated.Run.Block = cloneTemporalBlock(input.Projection.Block)
	updated.Run.Version++
	updated.Plan = input.Projection.Plan
	if current.Run.Status == imageagent.RunStatusFailed && input.Projection.Status == imageagent.RunStatusExecuting && input.CurrentNode == "execute_slots" {
		// A failed execution may already have committed slot results. Preserve
		// those logical facts while the new Temporal execution replays their
		// stable external-effect identities.
		updated.Slots = append([]imageagent.SlotProjection(nil), current.Slots...)
	} else {
		updated.Slots = append([]imageagent.SlotProjection(nil), input.Projection.Slots...)
	}
	updated.ResultDigest = input.Projection.ResultDigest
	updated.PendingCommand = clonePendingReceipt(input.Projection.PendingCommand)
	updated.RecoverableEffects = append([]imageagent.RecoverableEffect(nil), input.Projection.RecoverableEffects...)
	updated.CommandIngress = input.Projection.CommandIngress
	slotMutations, err := recoverySlotCodeMutations(current.Slots, updated.Slots, scope, input.PlanRevision, input.CurrentNode, input.CommitID)
	if err != nil {
		return err
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: input.CommitID, ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: updated, EventType: "run.updated", EventPayload: json.RawMessage(`{}`),
		ExpectedRunVersion: current.Run.Version,
		RunMutation:        &imageagent.RunMutation{Status: updated.Run.Status, CurrentNode: input.CurrentNode, ActivePlanRevision: input.PlanRevision, Block: updated.Run.Block},
		SlotMutations:      slotMutations,
	})
	if err != nil {
		return fmt.Errorf("commit image agent run projection: %w", err)
	}
	return nil
}

func recoverySlotCodeMutations(current, updated []imageagent.SlotProjection, scope imageagent.RunScope, planRevision int64, node, commitID string) ([]imageagent.SlotProjectionMutation, error) {
	if len(current) != len(updated) {
		return nil, imageagent.ErrRevisionConflict
	}
	mutations := make([]imageagent.SlotProjectionMutation, 0, len(updated))
	for index := range updated {
		before, after := current[index], updated[index]
		if reflect.DeepEqual(before, after) {
			continue
		}
		afterCode := after.ErrorCode
		before.ErrorCode, after.ErrorCode = "", ""
		if !reflect.DeepEqual(before, after) || after.Attempt <= 0 || after.Slot.Status != imageagent.SlotStatusBlocked || !imageagent.IsRecoverableEffectBlockCode(afterCode) {
			return nil, fmt.Errorf("%w: run-state persistence may only refresh recoverable slot codes", imageagent.ErrRevisionConflict)
		}
		candidateIDs := make([]string, 0, len(after.Candidates))
		for _, candidate := range after.Candidates {
			candidateIDs = append(candidateIDs, candidate.AssetID)
		}
		mutations = append(mutations, imageagent.SlotProjectionMutation{
			PlanRevision: planRevision,
			Result: imageagent.SlotResult{
				SlotID: after.Slot.ID, Attempt: after.Attempt, Status: after.Slot.Status,
				CandidateAssetIDs: candidateIDs, ErrorCode: afterCode,
			},
			Projection: updated[index],
			Attempt: imageagent.StepAttempt{
				TenantID: scope.TenantID, OwnerUserID: scope.OwnerUserID, RunID: scope.RunID, PlanRevision: planRevision,
				SlotID: after.Slot.ID, Attempt: after.Attempt, Node: node,
				IdempotencyKey: fmt.Sprintf("%s:slot:%s:attempt:%d", commitID, after.Slot.ID, after.Attempt),
				Outcome:        "blocked", ErrorCategory: afterCode,
			},
		})
	}
	return mutations, nil
}

func (a *Activities) PersistWorkflowFailure(ctx context.Context, input PersistWorkflowFailureActivityInput) error {
	return a.persistWorkflowFailure(ctx, input.RunID, input.Identity, input.FailureCode, input.FailureMessage, "workflow-failed")
}

func (a *Activities) PersistWorkflowFailureV2(ctx context.Context, input PersistWorkflowFailureV2ActivityInput) error {
	commitID := strings.TrimSpace(input.CommitID)
	if commitID == "" {
		return fmt.Errorf("workflow failure projection commit ID is required")
	}
	return a.persistWorkflowFailure(ctx, input.RunID, input.Identity, input.FailureCode, input.FailureMessage, commitID)
}

func (a *Activities) persistWorkflowFailure(ctx context.Context, runID string, identity imageagent.ExecutionIdentity, failureCode, failureMessage, commitID string) error {
	ctx, err := restoreActivityIdentity(ctx, identity)
	if err != nil {
		return err
	}
	if strings.TrimSpace(runID) == "" || failureCode != "workflow_failed" || strings.TrimSpace(failureMessage) == "" {
		return fmt.Errorf("persist workflow failure input is invalid")
	}
	scope := imageagent.RunScope{TenantID: identity.TenantID, OwnerUserID: identity.UserID, RunID: runID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return fmt.Errorf("get failed image agent projection: %w", err)
	}
	if isTerminalRunStatus(current.Run.Status) {
		return nil
	}
	block := &imageagent.Block{Code: failureCode, Message: failureMessage}
	updated := current
	updated.Run.Status = imageagent.RunStatusFailed
	updated.Run.CurrentNode = "workflow_failed"
	updated.Run.Block = block
	updated.Run.Version++
	eventPayload, err := json.Marshal(block)
	if err != nil {
		return fmt.Errorf("encode image agent workflow failure: %w", err)
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: commitID, ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: updated, EventType: "run.failed", EventPayload: eventPayload, ExpectedRunVersion: current.Run.Version,
		RunMutation: &imageagent.RunMutation{Status: imageagent.RunStatusFailed, CurrentNode: "workflow_failed", ActivePlanRevision: current.Run.ActivePlanRevision, Block: block},
	})
	if err != nil {
		return fmt.Errorf("commit failed image agent projection: %w", err)
	}
	return nil
}

func (a *Activities) PersistPlanRevision(ctx context.Context, input PersistPlanRevisionActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	if input.Plan.ParentRevision != input.ExpectedRevision || input.Plan.Revision <= input.ExpectedRevision || strings.TrimSpace(input.Plan.CreatedBy) != input.Identity.UserID {
		return fmt.Errorf("replacement plan revision, parent, and actor are invalid")
	}
	if err := imageagent.ValidatePlan(input.Plan); err != nil {
		return fmt.Errorf("validate replacement plan: %w", err)
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return err
	}
	if current.Plan.Revision == input.Plan.Revision && current.Run.ActivePlanRevision == input.Plan.Revision &&
		current.Run.Status == imageagent.RunStatusExecuting && reflect.DeepEqual(current.Plan, input.Plan) {
		return nil
	}
	updated := current
	updated.Plan = input.Plan
	updated.Run.ActivePlanRevision = input.Plan.Revision
	updated.Run.Status = imageagent.RunStatusExecuting
	updated.Run.CurrentNode = "execute_slots"
	updated.Run.Block = nil
	updated.Run.Version++
	updated.ResultDigest = ""
	updated.PendingCommand = nil
	updated.Slots = make([]imageagent.SlotProjection, len(input.Plan.Slots))
	for index, slot := range input.Plan.Slots {
		updated.Slots[index] = imageagent.SlotProjection{Slot: slot}
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "plan:" + input.Plan.IdempotencyKey, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: "plan.replaced", EventPayload: json.RawMessage(`{}`), ExpectedRunVersion: current.Run.Version, RunMutation: &imageagent.RunMutation{Status: imageagent.RunStatusExecuting, CurrentNode: "execute_slots", ActivePlanRevision: input.Plan.Revision}, PlanMutation: &imageagent.PlanProjectionMutation{ExpectedActiveRevision: input.ExpectedRevision, Plan: input.Plan}})
	if err != nil {
		return fmt.Errorf("commit replacement plan projection: %w", err)
	}
	return nil
}

func slotProjectionIndex(slots []imageagent.SlotProjection, slotID string) int {
	for index := range slots {
		if slots[index].Slot.ID == slotID {
			return index
		}
	}
	return -1
}
func cloneTemporalBlock(block *imageagent.Block) *imageagent.Block {
	if block == nil {
		return nil
	}
	cloned := *block
	return &cloned
}

func (a *Activities) PersistPendingCommand(ctx context.Context, input PersistPendingCommandActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	if strings.TrimSpace(input.CommitID) == "" {
		return fmt.Errorf("pending command projection commit ID is required")
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}
	current, err := a.repository.GetProjection(ctx, scope)
	if err != nil {
		return err
	}
	if reflect.DeepEqual(current.PendingCommand, input.Receipt) && reflect.DeepEqual(current.CommandIngress, input.CommandIngress) {
		return nil
	}
	updated := current
	updated.PendingCommand = clonePendingReceipt(input.Receipt)
	updated.CommandIngress = input.CommandIngress
	eventType := "command.receipt.updated"
	if input.CommandIngress.Exhausted && !current.CommandIngress.Exhausted {
		eventType = "command.ingress.exhausted"
	}
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: input.CommitID, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: eventType, EventPayload: json.RawMessage(`{}`)})
	return err
}

func clonePendingReceipt(receipt *imageagent.PendingCommandReceipt) *imageagent.PendingCommandReceipt {
	if receipt == nil {
		return nil
	}
	cloned := *receipt
	return &cloned
}
