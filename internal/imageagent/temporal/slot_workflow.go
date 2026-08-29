package temporal

import (
	"errors"
	"fmt"
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
)

func ImageSlotWorkflow(ctx workflow.Context, input SlotWorkflowInput) (SlotWorkflowResult, error) {
	activityName := activityWireForFrozenSlotWorkflow(ctx)
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
	if err := workflow.ExecuteActivity(ctx, activityName, activityInput).Get(ctx, &execution); err != nil {
		return SlotWorkflowResult{
			Execution: imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
			Status:    imageagent.SlotStatusBlocked, ErrorCode: slotExecutionErrorCode(err),
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

func activityWireForFrozenSlotWorkflow(ctx workflow.Context) string {
	if workflow.GetVersion(ctx, activityWireV2Patch, workflow.DefaultVersion, 1) == workflow.DefaultVersion {
		return activityExecuteSlotLegacy
	}
	return activityExecuteSlot
}

func slotExecutionErrorCode(err error) string {
	var applicationError *sdktemporal.ApplicationError
	if errors.As(err, &applicationError) && applicationError.Type() == slotProviderOutcomeUnknownErrorType {
		return "slot_provider_outcome_unknown"
	}
	return "slot_execution_failed"
}

// ImageSlotWorkflowV3 is additive and intentionally not registered by Task 4.
// The v2 child workflow above remains byte-for-byte compatible with histories
// that execute imageagent.execute_slot.v2.
func ImageSlotWorkflowV3(ctx workflow.Context, input SlotWorkflowV3Input) (SlotWorkflowV3Result, error) {
	activityName := strings.TrimSpace(input.ExecuteActivityName)
	if activityName == "" {
		return SlotWorkflowV3Result{}, fmt.Errorf("v3 execute activity name is required")
	}
	startToClose := 10 * time.Minute
	if !input.LifecycleDeadlineAt.IsZero() {
		providerWindow := input.LifecycleDeadlineAt.Sub(workflow.Now(ctx))
		if providerWindow <= 0 {
			return SlotWorkflowV3Result{
				Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
				Status:    imageagent.SlotStatusBlocked, ErrorCode: imageagent.WorkflowLifecycleElapsedCode,
				EffectPhase: effectPhaseForFinalizationWire(input.ExternalEffectFinalization, imageagent.SlotEffectV3ProviderNotDispatched),
			}, nil
		}
		startToClose = min(startToClose, providerWindow)
	}
	if input.BudgetAuthorization && !input.DeadlineAt.IsZero() {
		providerWindow := input.DeadlineAt.Sub(workflow.Now(ctx))
		if providerWindow <= 0 {
			return SlotWorkflowV3Result{
				Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
				Status:    imageagent.SlotStatusBlocked, ErrorCode: imageagent.BudgetElapsedCode,
				EffectPhase: effectPhaseForFinalizationWire(input.ExternalEffectFinalization, imageagent.SlotEffectV3ProviderNotDispatched),
			}, nil
		}
		if input.ExternalEffectFinalization {
			startToClose = addFinalizationGrace(providerWindow)
		} else if providerWindow < startToClose {
			startToClose = providerWindow
		}
	}
	ctx = workflow.WithActivityOptions(ctx, slotWorkflowV3ActivityOptions(startToClose, input.ExternalEffectFinalization))
	activityInput := ExecuteSlotV3ActivityInput{
		RunID: input.RunID, Identity: input.Identity, PlanRevision: input.PlanRevision,
		Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey:             slotAttemptKey(input.PlanRevision, input.Slot, input.Attempt),
		AssetCatalog:               input.AssetCatalog,
		ExternalEffectFinalization: input.ExternalEffectFinalization,
		BudgetAuthorization:        input.BudgetAuthorization, BudgetPolicy: input.BudgetPolicy, DeadlineAt: input.DeadlineAt, LifecycleDeadlineAt: input.LifecycleDeadlineAt,
	}
	var published imageagent.SlotEffectV3PublishedResult
	for {
		err := workflow.ExecuteActivity(ctx, activityName, activityInput).Get(ctx, &published)
		if err == nil {
			break
		}
		retryDelay, recoverPublication := slotPublicationRecoveryDelay(err)
		if !recoverPublication {
			code := slotExecutionV3ErrorCode(err)
			return SlotWorkflowV3Result{
				Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
				Status:    imageagent.SlotStatusBlocked, ErrorCode: code,
				EffectPhase: effectPhaseForFinalizationWire(input.ExternalEffectFinalization, terminalEffectPhaseForErrorCode(code)),
			}, nil
		}
		if err := workflow.Sleep(ctx, retryDelay); err != nil {
			if input.ExternalEffectFinalization {
				return SlotWorkflowV3Result{
					Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
					Status:    imageagent.SlotStatusBlocked, ErrorCode: "slot_execution_failed",
					EffectPhase: imageagent.SlotEffectV3PublicationClaimed,
				}, nil
			}
			return SlotWorkflowV3Result{}, err
		}
	}
	if input.Slot.Role == imageagent.SlotRoleMain && len(published.Candidates) != 1 {
		return SlotWorkflowV3Result{
			Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
			Status:    imageagent.SlotStatusBlocked, ErrorCode: invalidMainCandidateCountCode,
			EffectPhase: effectPhaseForFinalizationWire(input.ExternalEffectFinalization, imageagent.SlotEffectV3PublicationComplete),
		}, nil
	}
	normalized, err := imageagent.NormalizeSlotEffectV3PublishedResult(published)
	if err != nil || normalized.SlotID != input.Slot.ID || normalized.Attempt != input.Attempt {
		return SlotWorkflowV3Result{
			Published: imageagent.SlotEffectV3PublishedResult{SlotID: input.Slot.ID, Attempt: input.Attempt},
			Status:    imageagent.SlotStatusBlocked, ErrorCode: "invalid_slot_result",
			EffectPhase: effectPhaseForFinalizationWire(input.ExternalEffectFinalization, imageagent.SlotEffectV3PublicationComplete),
		}, nil
	}
	return SlotWorkflowV3Result{
		Published: normalized, Status: imageagent.SlotStatusAccepted,
		EffectPhase: effectPhaseForFinalizationWire(input.ExternalEffectFinalization, imageagent.SlotEffectV3PublicationComplete),
	}, nil
}

func effectPhaseForFinalizationWire(enabled bool, phase imageagent.SlotEffectV3Phase) imageagent.SlotEffectV3Phase {
	if !enabled {
		return ""
	}
	return phase
}

func terminalEffectPhaseForErrorCode(code string) imageagent.SlotEffectV3Phase {
	switch code {
	case imageagent.SlotProviderNotDispatchedCode, imageagent.BudgetExhaustedCode, imageagent.BudgetQuoteUnavailableCode, imageagent.BudgetElapsedCode, imageagent.WorkflowLifecycleElapsedCode:
		return imageagent.SlotEffectV3ProviderNotDispatched
	case imageagent.SlotProviderOutcomeUnknownCode:
		return imageagent.SlotEffectV3ProviderUnknown
	case imageagent.SlotStagingOutcomeUnknownCode:
		return imageagent.SlotEffectV3StagingUnknown
	case imageagent.SlotPublicationOutcomeUnknownCode:
		return imageagent.SlotEffectV3PublicationUnknown
	default:
		return ""
	}
}

func addFinalizationGrace(providerWindow time.Duration) time.Duration {
	const maxDuration = time.Duration(1<<63 - 1)
	if providerWindow > maxDuration-providerFinalizationTimeout {
		return maxDuration
	}
	return providerWindow + providerFinalizationTimeout
}

func slotWorkflowV3ActivityOptions(startToClose time.Duration, externalEffectFinalization bool) workflow.ActivityOptions {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: startToClose,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2,
			MaximumInterval: 30 * time.Second, MaximumAttempts: 5,
			NonRetryableErrorTypes: []string{slotPublicationRecoveryErrorType},
		},
	}
	if externalEffectFinalization {
		options.HeartbeatTimeout = externalEffectHeartbeatTimeout
		options.WaitForCancellation = true
	}
	return options
}

func slotPublicationRecoveryDelay(err error) (time.Duration, bool) {
	var applicationError *sdktemporal.ApplicationError
	if !errors.As(err, &applicationError) || applicationError.Type() != slotPublicationRecoveryErrorType {
		return 0, false
	}
	var details slotPublicationRecoveryDetails
	if detailsErr := applicationError.Details(&details); detailsErr != nil || details.RetryDelay <= 0 {
		return 0, false
	}
	return details.RetryDelay, true
}

func slotExecutionV3ErrorCode(err error) string {
	var applicationError *sdktemporal.ApplicationError
	if errors.As(err, &applicationError) {
		switch applicationError.Type() {
		case imageagent.SlotProviderNotDispatchedCode, slotProviderOutcomeUnknownCode, slotStagingOutcomeUnknownCode, slotPublicationOutcomeUnknownCode:
			return applicationError.Type()
		case slotEffectPhaseInvalidCode:
			return imageagent.SlotEffectPhaseInvalidCode
		case slotEffectPolicyInvalidCode:
			return imageagent.SlotEffectPolicyInvalidCode
		case imageagent.BudgetExhaustedCode, imageagent.BudgetQuoteUnavailableCode, imageagent.BudgetElapsedCode, imageagent.WorkflowLifecycleElapsedCode:
			return applicationError.Type()
		default:
			if strings.HasPrefix(applicationError.Type(), "imageagent_slot_effect_") {
				return imageagent.SlotEffectPolicyInvalidCode
			}
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
