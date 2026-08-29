package temporal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func TestRegisterWorkerIncludesEffectRecoveryWorkflowAndActivity(t *testing.T) {
	input := EffectRecoveryWorkflowInput{
		RunID:        "run-1",
		Identity:     imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		PlanRevision: 1,
		Slot:         sevenSlotPlan().Slots[0],
		Attempt:      1,
	}
	raw := &recordingSDKClient{}
	starter := newRecoveryWorkflowStarter(raw, TaskQueueV3)

	require.NoError(t, starter(context.Background(), input))
	require.Equal(t, "image-agent-effect-recovery:tenant-a:user-a:run-1:1:slot-1:1", raw.startOptions.ID)
	require.Equal(t, TaskQueueV3, raw.startOptions.TaskQueue)
	require.Equal(t, enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, raw.startOptions.WorkflowIDConflictPolicy)
	require.Equal(t, enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY, raw.startOptions.WorkflowIDReusePolicy)
	require.Equal(t, EffectRecoveryWorkflowName, raw.workflowName)
	require.Equal(t, input, raw.effectRecoveryInput)

	activities, err := NewActivities(ActivityDependencies{
		Repository:   store.NewMemoryRepository(),
		SlotExecutor: &identityCheckingExecutor{t: t},
		Publisher:    &identityCheckingPublisher{t: t},
	})
	require.NoError(t, err)
	registrar := &recordingWorkerRegistrar{}

	require.NoError(t, RegisterWorkerForMode(registrar, activities, WorkerWireModeV3))
	require.Contains(t, registrar.workflows, EffectRecoveryWorkflowName)
	require.Contains(t, registrar.activities, activityStartEffectRecoveryV3)
	require.Contains(t, registrar.activities, activityRecoverEffectV3)
	require.Contains(t, registrar.activities, activityPersistRecoveryBlockedV3)
}
