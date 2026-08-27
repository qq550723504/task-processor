package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
)

const slotResultPersistedEventType = "slot.result.persisted"

type ActivityDependencies struct {
	Repository   imageagent.Repository
	SlotExecutor imageagent.SlotExecutor
	Publisher    imageagent.ApprovedAssetPublisher
}

type Activities struct {
	repository   imageagent.Repository
	slotExecutor imageagent.SlotExecutor
	publisher    imageagent.ApprovedAssetPublisher
}

func NewActivities(dependencies ActivityDependencies) (*Activities, error) {
	if dependencies.Repository == nil {
		return nil, fmt.Errorf("image agent repository is required")
	}
	if dependencies.SlotExecutor == nil {
		return nil, fmt.Errorf("image agent slot executor is required")
	}
	if dependencies.Publisher == nil {
		return nil, fmt.Errorf("image agent approved asset publisher is required")
	}
	return &Activities{repository: dependencies.Repository, slotExecutor: dependencies.SlotExecutor, publisher: dependencies.Publisher}, nil
}

func (a *Activities) ExecuteSlot(ctx context.Context, input ExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return a.slotExecutor.ExecuteSlot(ctx, imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: input.IdempotencyKey,
		AssetCatalog:   input.AssetCatalog,
	})
}

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
		reflect.DeepEqual(current.PendingCommand, input.Projection.PendingCommand) {
		return nil
	}
	updated := current
	updated.Run.Status = input.Projection.Status
	updated.Run.CurrentNode = input.CurrentNode
	updated.Run.Block = cloneTemporalBlock(input.Projection.Block)
	updated.Run.Version++
	updated.Plan = input.Projection.Plan
	updated.Slots = append([]imageagent.SlotProjection(nil), input.Projection.Slots...)
	updated.ResultDigest = input.Projection.ResultDigest
	updated.PendingCommand = clonePendingReceipt(input.Projection.PendingCommand)
	updated.CommandIngress = input.Projection.CommandIngress
	_, err = a.repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: input.CommitID, ExpectedProjectionVersion: current.ProjectionVersion, Snapshot: updated, EventType: "run.updated", EventPayload: json.RawMessage(`{}`), ExpectedRunVersion: current.Run.Version, RunMutation: &imageagent.RunMutation{Status: updated.Run.Status, CurrentNode: input.CurrentNode, ActivePlanRevision: input.PlanRevision, Block: updated.Run.Block}})
	if err != nil {
		return fmt.Errorf("commit image agent run projection: %w", err)
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

func (a *Activities) PublishApproved(ctx context.Context, input PublishApprovedActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	return a.publisher.PublishApproved(ctx, imageagent.PublishApprovedInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, CandidateAssetIDs: append([]string(nil), input.CandidateAssetIDs...),
		IdempotencyKey: input.IdempotencyKey,
	})
}

func legacyMigrationRequiredError() error {
	return sdktemporal.NewNonRetryableApplicationError(
		"legacy image agent workflow requires an explicit v2 migration or a new run",
		updateErrorLegacyMigrationRequired,
		nil,
	)
}

func (a *Activities) LegacyExecuteSlot(context.Context, LegacyExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistSlotResult(context.Context, LegacyPersistSlotResultActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistRunState(context.Context, LegacyPersistRunStateActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistPlanRevision(context.Context, LegacyPersistPlanRevisionActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPersistPendingCommand(context.Context, LegacyPersistPendingCommandActivityInput) error {
	return legacyMigrationRequiredError()
}

func (a *Activities) LegacyPublishApproved(context.Context, LegacyPublishApprovedActivityInput) error {
	return legacyMigrationRequiredError()
}

type activityRegistrar interface {
	RegisterActivityWithOptions(interface{}, sdkactivity.RegisterOptions)
}

func RegisterActivities(registrar activityRegistrar, activities *Activities) error {
	if registrar == nil {
		return fmt.Errorf("temporal activity registrar is required")
	}
	if activities == nil {
		return fmt.Errorf("image agent activities are required")
	}
	registrar.RegisterActivityWithOptions(activities.LegacyExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlotLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResultLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunStateLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevisionLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommandLegacy})
	registrar.RegisterActivityWithOptions(activities.LegacyPublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApprovedLegacy})
	registrar.RegisterActivityWithOptions(activities.ExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlot})
	registrar.RegisterActivityWithOptions(activities.PersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResult})
	registrar.RegisterActivityWithOptions(activities.PersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunState})
	registrar.RegisterActivityWithOptions(activities.PersistPlanRevision, sdkactivity.RegisterOptions{Name: activityPersistPlanRevision})
	registrar.RegisterActivityWithOptions(activities.PersistPendingCommand, sdkactivity.RegisterOptions{Name: activityPersistPendingCommand})
	registrar.RegisterActivityWithOptions(activities.PublishApproved, sdkactivity.RegisterOptions{Name: activityPublishApproved})
	return nil
}

func restoreActivityIdentity(ctx context.Context, identity imageagent.ExecutionIdentity) (context.Context, error) {
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	if identity.TenantID == "" || identity.UserID == "" {
		return nil, fmt.Errorf("captured image agent tenant and user identity are required")
	}
	return authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: identity.TenantID, UserID: identity.UserID}), nil
}
