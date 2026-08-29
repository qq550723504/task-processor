package temporal

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

func TestEffectRecoveryWorkflowUsesDeterministicIDAndAttachesDuplicateStart(t *testing.T) {
	_, input := initializedSlotEffectV3Activity(t, "run-v3-recovery-id")

	duplicateKey := input.RunID + ":" + input.Slot.ID
	first := EffectRecoveryWorkflowID(input.Identity, input.PlanRevision, duplicateKey, input.Attempt)
	second := EffectRecoveryWorkflowID(input.Identity, input.PlanRevision, duplicateKey, input.Attempt)
	nextAttempt := EffectRecoveryWorkflowID(input.Identity, input.PlanRevision, duplicateKey, input.Attempt+1)

	require.Equal(t, "image-agent-effect-recovery:tenant-a:user-a:run-v3-recovery-id:1:slot-1:1", first)
	require.Equal(t, first, second, "duplicate starts must target the same deterministic recovery workflow ID")
	require.NotEqual(t, first, nextAttempt)
}

func TestEffectRecoveryWorkflowPersistsRecoveryBlockedAfterBoundedExhaustion(t *testing.T) {
	_, input := initializedSlotEffectV3Activity(t, "run-v3-recovery-blocked")
	var attempts int
	env := newEffectRecoveryWorkflowEnvWithRecovery(t, func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error) {
		attempts++
		return EffectRecoveryResult{}, errors.New("durable repository unavailable")
	})

	env.ExecuteWorkflow(ImageAgentEffectRecoveryWorkflow, effectRecoveryWorkflowInput(input))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 3, attempts, "recovery must stop after bounded workflow activity retries")
	var result EffectRecoveryResult
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, EffectRecoveryOutcomeRecoveryBlocked, result.Outcome)
	require.Equal(t, effectRecoveryBlockedCode, result.BlockedCode)
	require.Equal(t, input.Slot.ID, result.Published.SlotID)
	require.Equal(t, input.Attempt, result.Published.Attempt)
}

func newEffectRecoveryWorkflowEnv(t *testing.T, activities *Activities) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	return newEffectRecoveryWorkflowEnvWithRecovery(t, activities.RecoverEffectV3)
}

func newEffectRecoveryWorkflowEnvWithRecovery(t *testing.T, recover func(context.Context, EffectRecoveryWorkflowInput) (EffectRecoveryResult, error)) *testsuite.TestWorkflowEnvironment {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImageAgentEffectRecoveryWorkflow)
	env.RegisterActivityWithOptions(recover, sdkactivity.RegisterOptions{Name: activityRecoverEffectV3})
	return env
}

func effectRecoveryWorkflowInput(input ExecuteSlotV3ActivityInput) EffectRecoveryWorkflowInput {
	return EffectRecoveryWorkflowInput{
		RunID:        input.RunID,
		Identity:     input.Identity,
		PlanRevision: input.PlanRevision,
		Slot:         input.Slot,
		Attempt:      input.Attempt,
		AssetCatalog: input.AssetCatalog,
	}
}
