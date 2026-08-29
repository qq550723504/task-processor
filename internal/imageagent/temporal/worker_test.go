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

func TestTemporalClientRecoverEffectUsesDeterministicWorkflowID(t *testing.T) {
	raw := &recordingSDKClient{}
	client := NewClient(raw)
	projection := imageagent.RunProjection{
		Run:  imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual},
		Plan: sevenSlotPlan(),
		AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-1.png"},
		}},
	}

	err := client.RecoverEffect(context.Background(), imageagent.RecoverEffectCommand{
		RunID: "run-1", PlanRevision: 1, SlotID: "slot-1", Attempt: 1, ActionID: "recover-1",
		Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
	}, projection)

	require.NoError(t, err)
	require.Equal(t, "image-agent-effect-recovery:tenant-a:user-a:run-1:1:slot-1:1", raw.startOptions.ID)
	require.Equal(t, TaskQueueV3, raw.startOptions.TaskQueue)
	require.Equal(t, enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, raw.startOptions.WorkflowIDConflictPolicy)
	require.Equal(t, enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY, raw.startOptions.WorkflowIDReusePolicy)
	require.Equal(t, EffectRecoveryWorkflowName, raw.workflowName)
	require.Equal(t, projection.AssetCatalog, raw.effectRecoveryInput.AssetCatalog)
	require.Equal(t, sevenSlotPlan().Slots[0], raw.effectRecoveryInput.Slot)
	require.Equal(t, 1, raw.effectRecoveryInput.Attempt)
}
