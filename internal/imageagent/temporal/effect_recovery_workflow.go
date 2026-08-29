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

const (
	effectRecoveryPublicationRetryAttempts = 3
	effectRecoveryPublicationRetryWindow   = 5 * time.Minute
	effectRecoveryParentSignalPatch        = "image-agent-effect-recovery-parent-signal-v1"
)

func ImageAgentEffectRecoveryWorkflow(ctx workflow.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &sdktemporal.RetryPolicy{
			InitialInterval: time.Second, BackoffCoefficient: 2,
			MaximumInterval: 30 * time.Second, MaximumAttempts: 3,
			NonRetryableErrorTypes: []string{slotPublicationRecoveryErrorType},
		},
	})
	retryDeadline := workflow.Now(ctx).Add(effectRecoveryPublicationRetryWindow)
	publicationRecoveryAttempts := 0
	for {
		var result EffectRecoveryResult
		err := workflow.ExecuteActivity(ctx, activityRecoverEffectV3, input).Get(ctx, &result)
		if err == nil {
			return reconcileEffectRecovery(ctx, input)
		}
		retryDelay, recoverPublication := slotPublicationRecoveryDelay(err)
		if !recoverPublication {
			return persistEffectRecoveryBlocked(ctx, input)
		}
		publicationRecoveryAttempts++
		if publicationRecoveryAttempts >= effectRecoveryPublicationRetryAttempts || !workflow.Now(ctx).Add(retryDelay).Before(retryDeadline) {
			return persistEffectRecoveryBlocked(ctx, input)
		}
		if sleepErr := workflow.Sleep(ctx, retryDelay); sleepErr != nil {
			return persistEffectRecoveryBlocked(ctx, input)
		}
	}
}

func effectRecoveryBlockedResult(input EffectRecoveryWorkflowInput) EffectRecoveryResult {
	return EffectRecoveryResult{
		Outcome: EffectRecoveryOutcomeRecoveryBlocked,
		Published: imageagent.SlotEffectV3PublishedResult{
			SlotID:  strings.TrimSpace(input.Slot.ID),
			Attempt: input.Attempt,
		},
		EffectPhase: imageagent.SlotEffectV3RecoveryBlocked,
		BlockedCode: effectRecoveryBlockedCode,
	}
}

func persistEffectRecoveryBlocked(ctx workflow.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	var result EffectRecoveryResult
	if err := workflow.ExecuteActivity(ctx, activityPersistRecoveryBlockedV3, input).Get(ctx, &result); err != nil {
		return EffectRecoveryResult{}, err
	}
	return reconcileEffectRecovery(ctx, input)
}

func reconcileEffectRecovery(ctx workflow.Context, input EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
	var result EffectRecoveryResult
	if err := workflow.ExecuteActivity(ctx, activityReconcileEffectRecoveryV3, input).Get(ctx, &result); err != nil {
		return EffectRecoveryResult{}, err
	}
	if workflow.GetVersion(ctx, effectRecoveryParentSignalPatch, workflow.DefaultVersion, 1) != workflow.DefaultVersion {
		if err := signalEffectRecoveryCompletion(ctx, input, result); err != nil {
			return EffectRecoveryResult{}, err
		}
	}
	return result, nil
}

func signalEffectRecoveryCompletion(ctx workflow.Context, input EffectRecoveryWorkflowInput, result EffectRecoveryResult) error {
	parentID := WorkflowID(input.Identity.TenantID, input.Identity.UserID, input.RunID)
	err := workflow.SignalExternalWorkflow(ctx, parentID, "", signalEffectRecoveryCompleted, EffectRecoveryCompletedSignal{
		RunID: input.RunID, PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt, Result: result,
	}).Get(ctx, nil)
	if err == nil {
		return nil
	}
	var unknown *sdktemporal.UnknownExternalWorkflowExecutionError
	if errors.As(err, &unknown) {
		// Durable reconciliation is authoritative when the parent has already
		// completed (or is absent in a historical/replay-only test).
		return nil
	}
	return fmt.Errorf("signal parent recovery completion: %w", err)
}
