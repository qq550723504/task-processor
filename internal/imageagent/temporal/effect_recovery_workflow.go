package temporal

import (
	"strings"
	"time"

	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
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
	for {
		var result EffectRecoveryResult
		err := workflow.ExecuteActivity(ctx, activityRecoverEffectV3, input).Get(ctx, &result)
		if err == nil {
			return result, nil
		}
		retryDelay, recoverPublication := slotPublicationRecoveryDelay(err)
		if !recoverPublication {
			return effectRecoveryBlockedResult(input), nil
		}
		if sleepErr := workflow.Sleep(ctx, retryDelay); sleepErr != nil {
			return effectRecoveryBlockedResult(input), nil
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
		BlockedCode: effectRecoveryBlockedCode,
	}
}
