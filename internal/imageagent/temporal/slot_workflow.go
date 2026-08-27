package temporal

import (
	"errors"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

func ImageSlotWorkflow(ctx workflow.Context, input SlotWorkflowInput) (SlotWorkflowResult, error) {
	activityWire := activityWireForWorkflow(ctx)
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    3,
		},
	})
	activityInput := ExecuteSlotActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.PlanRevision,
		Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:   input.AssetCatalog,
	}
	var execution imageagent.SlotExecutionResult
	if err := workflow.ExecuteActivity(ctx, activityWire.executeSlot, activityInput).Get(ctx, &execution); err != nil {
		return SlotWorkflowResult{
			Execution: imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
			Status:    imageagent.SlotStatusBlocked, ErrorCode: slotExecutionErrorCode(err),
		}, nil
	}
	if input.Slot.Role == imageagent.SlotRoleMain && len(execution.Candidates) != 1 {
		return SlotWorkflowResult{
			Execution: imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
			Status:    imageagent.SlotStatusBlocked, ErrorCode: invalidMainCandidateCountCode,
		}, nil
	}
	if execution.SlotID != input.Slot.ID || execution.Attempt != input.Attempt || !hasCandidateAsset(execution.Candidates) {
		return SlotWorkflowResult{
			Execution: imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
			Status:    imageagent.SlotStatusBlocked, ErrorCode: "invalid_slot_result",
		}, nil
	}
	return SlotWorkflowResult{Execution: execution, Status: imageagent.SlotStatusAccepted}, nil
}

func slotExecutionErrorCode(err error) string {
	var applicationError *sdktemporal.ApplicationError
	if errors.As(err, &applicationError) {
		switch applicationError.Type() {
		case slotProviderOutcomeUnknownErrorType, slotProviderOutcomeUnknownCode:
			return slotProviderOutcomeUnknownCode
		case slotStagingOutcomeUnknownCode:
			return slotStagingOutcomeUnknownCode
		case slotPublicationOutcomeUnknownCode:
			return slotPublicationOutcomeUnknownCode
		}
	}
	return "slot_execution_failed"
}

func hasCandidateAsset(candidates []imageagent.AssetCandidate) bool {
	if len(candidates) == 0 {
		return false
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.AssetID) == "" {
			return false
		}
	}
	return true
}
