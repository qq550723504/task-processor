package temporal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	sdkactivity "go.temporal.io/sdk/activity"

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
	candidateIDs := make([]string, 0, len(result.Execution.Candidates))
	for _, candidate := range result.Execution.Candidates {
		id := strings.TrimSpace(candidate.AssetID)
		if id == "" {
			if result.Status == imageagent.SlotStatusAccepted {
				return fmt.Errorf("accepted slot result contains an empty candidate asset ID")
			}
			continue
		}
		candidateIDs = append(candidateIDs, id)
	}
	if result.Status == imageagent.SlotStatusAccepted && len(candidateIDs) == 0 {
		return fmt.Errorf("accepted slot result requires at least one candidate asset ID")
	}
	outcome := "accepted"
	if result.Status != imageagent.SlotStatusAccepted {
		outcome = "blocked"
	}
	attempt := imageagent.StepAttempt{
		TenantID: input.Identity.TenantID, RunID: input.RunID,
		SlotID: result.Execution.SlotID, Node: "execute_slot",
		IdempotencyKey: input.AttemptKey, Attempt: result.Execution.Attempt,
		Outcome: outcome, ErrorCategory: result.ErrorCode,
	}
	if err := a.repository.AppendAttempt(ctx, attempt); err != nil {
		return fmt.Errorf("append slot attempt: %w", err)
	}
	if err := a.repository.SaveSlotResult(ctx, imageagent.RunScope{TenantID: input.Identity.TenantID, RunID: input.RunID}, input.PlanRevision, imageagent.SlotResult{
		SlotID: result.Execution.SlotID, Attempt: result.Execution.Attempt,
		Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode,
	}); err != nil {
		return fmt.Errorf("save terminal slot result: %w", err)
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, RunID: input.RunID}
	eventPayload, err := json.Marshal(slotResultPersistedEventPayload{
		PlanRevision: input.PlanRevision, SlotID: result.Execution.SlotID,
		Attempt: result.Execution.Attempt, AttemptKey: input.AttemptKey,
		Status: result.Status, CandidateAssetIDs: candidateIDs, ErrorCode: result.ErrorCode,
	})
	if err != nil {
		return fmt.Errorf("encode terminal slot result event: %w", err)
	}
	run, err := a.repository.GetRun(ctx, scope)
	if err != nil {
		return fmt.Errorf("get image agent run for slot result event: %w", err)
	}
	if err := appendSlotResultEvent(ctx, a.repository, scope, run.Version, eventPayload); err != nil {
		return err
	}
	return nil
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

func appendSlotResultEvent(ctx context.Context, repository imageagent.Repository, scope imageagent.RunScope, projectionVersion int64, payload json.RawMessage) error {
	const pageSize = 100
	var afterCursor int64
	for {
		events, err := repository.ListEvents(ctx, scope, afterCursor, pageSize)
		if err != nil {
			return fmt.Errorf("list terminal slot result events: %w", err)
		}
		for _, event := range events {
			if event.Type == slotResultPersistedEventType && bytes.Equal(event.Payload, payload) {
				return nil
			}
		}
		if len(events) == 0 {
			break
		}
		afterCursor = events[len(events)-1].Cursor
		if len(events) < pageSize {
			break
		}
	}
	err := repository.AppendEvent(ctx, imageagent.RunEvent{
		TenantID: scope.TenantID, RunID: scope.RunID, Type: slotResultPersistedEventType,
		Cursor: afterCursor + 1, ProjectionVersion: projectionVersion, Payload: append(json.RawMessage(nil), payload...),
	})
	if errors.Is(err, imageagent.ErrRevisionConflict) {
		events, listErr := repository.ListEvents(ctx, scope, afterCursor, pageSize)
		if listErr != nil {
			return fmt.Errorf("verify terminal slot result event after cursor conflict: %w", listErr)
		}
		for _, event := range events {
			if event.Type == slotResultPersistedEventType && bytes.Equal(event.Payload, payload) {
				return nil
			}
		}
	}
	if err != nil {
		return fmt.Errorf("append terminal slot result event: %w", err)
	}
	return nil
}

func (a *Activities) PersistRunState(ctx context.Context, input PersistRunStateActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	scope := imageagent.RunScope{TenantID: input.Identity.TenantID, RunID: input.RunID}
	run, err := a.repository.GetRun(ctx, scope)
	if err != nil {
		return fmt.Errorf("get image agent run: %w", err)
	}
	if run.ActivePlanRevision != input.PlanRevision {
		return imageagent.ErrRevisionConflict
	}
	if run.Status == input.Status && run.CurrentNode == input.CurrentNode && reflect.DeepEqual(run.Block, input.Block) {
		return nil
	}
	if err := a.repository.UpdateRun(ctx, scope, run.Version, imageagent.RunMutation{
		Status: input.Status, CurrentNode: input.CurrentNode,
		ActivePlanRevision: input.PlanRevision, Block: input.Block,
	}); err != nil {
		return fmt.Errorf("update image agent run: %w", err)
	}
	return nil
}

func (a *Activities) PublishApproved(ctx context.Context, input PublishApprovedActivityInput) error {
	ctx, err := restoreActivityIdentity(ctx, input.Identity)
	if err != nil {
		return err
	}
	return a.publisher.PublishApproved(ctx, imageagent.PublishApprovedInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID,
		PlanRevision: input.PlanRevision, CandidateAssetIDs: append([]string(nil), input.CandidateAssetIDs...),
		IdempotencyKey: input.IdempotencyKey,
	})
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
	registrar.RegisterActivityWithOptions(activities.ExecuteSlot, sdkactivity.RegisterOptions{Name: activityExecuteSlot})
	registrar.RegisterActivityWithOptions(activities.PersistSlotResult, sdkactivity.RegisterOptions{Name: activityPersistSlotResult})
	registrar.RegisterActivityWithOptions(activities.PersistRunState, sdkactivity.RegisterOptions{Name: activityPersistRunState})
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
