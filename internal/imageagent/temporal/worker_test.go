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
		Run:  imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, Status: imageagent.RunStatusBlocked, Block: &imageagent.Block{Code: "recovery_start_failed", SlotID: "slot-1"}},
		Plan: sevenSlotPlan(),
		Slots: []imageagent.SlotProjection{{
			Slot:    imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusBlocked},
			Attempt: 1, ErrorCode: "recovery_start_failed",
		}},
		AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-1.png"},
		}},
	}

	err := client.RecoverEffect(context.Background(), imageagent.RecoverEffectCommand{
		RunID: "run-1", PlanRevision: 1, SlotID: "slot-1", Attempt: 1, ActionID: "recover-1",
		Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, Projection: projection,
	})

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

func TestTemporalClientRecoverEffectRejectsMismatchedProjectionBeforeStartingWorkflow(t *testing.T) {
	tests := []struct {
		name    string
		command imageagent.RecoverEffectCommand
		wantErr error
	}{
		{
			name: "non blocked status",
			command: imageagent.RecoverEffectCommand{
				RunID: "run-1", PlanRevision: 1, SlotID: "slot-1", Attempt: 1, ActionID: "recover-1",
				Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
				Projection: imageagent.RunProjection{
					Run:   imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, Status: imageagent.RunStatusExecuting, Block: &imageagent.Block{Code: "recovery_start_failed", SlotID: "slot-1"}},
					Plan:  sevenSlotPlan(),
					Slots: []imageagent.SlotProjection{{Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusBlocked}, Attempt: 1, ErrorCode: "recovery_start_failed"}},
				},
			},
			wantErr: imageagent.ErrCommandBlocked,
		},
		{
			name: "stale revision",
			command: imageagent.RecoverEffectCommand{
				RunID: "run-1", PlanRevision: 1, SlotID: "slot-1", Attempt: 1, ActionID: "recover-1",
				Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
				Projection: imageagent.RunProjection{
					Run:   imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, Status: imageagent.RunStatusBlocked, Block: &imageagent.Block{Code: "recovery_start_failed", SlotID: "slot-1"}},
					Plan:  imageagent.Plan{Revision: 2, IdempotencyKey: sevenSlotPlan().IdempotencyKey, SourceAssetIDs: sevenSlotPlan().SourceAssetIDs, Slots: sevenSlotPlan().Slots},
					Slots: []imageagent.SlotProjection{{Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusBlocked}, Attempt: 1, ErrorCode: "recovery_start_failed"}},
				},
			},
			wantErr: imageagent.ErrRevisionConflict,
		},
		{
			name: "wrong attempt projection",
			command: imageagent.RecoverEffectCommand{
				RunID: "run-1", PlanRevision: 1, SlotID: "slot-1", Attempt: 1, ActionID: "recover-1",
				Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
				Projection: imageagent.RunProjection{
					Run:   imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, Status: imageagent.RunStatusBlocked, Block: &imageagent.Block{Code: "recovery_start_failed", SlotID: "slot-1"}},
					Plan:  sevenSlotPlan(),
					Slots: []imageagent.SlotProjection{{Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusBlocked}, Attempt: 2, ErrorCode: "recovery_start_failed"}},
				},
			},
			wantErr: imageagent.ErrCommandBlocked,
		},
		{
			name: "non recovery block code",
			command: imageagent.RecoverEffectCommand{
				RunID: "run-1", PlanRevision: 1, SlotID: "slot-1", Attempt: 1, ActionID: "recover-1",
				Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
				Projection: imageagent.RunProjection{
					Run:   imageagent.Run{ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, Status: imageagent.RunStatusBlocked, Block: &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"}},
					Plan:  sevenSlotPlan(),
					Slots: []imageagent.SlotProjection{{Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusBlocked}, Attempt: 1, ErrorCode: "slot_failed"}},
				},
			},
			wantErr: imageagent.ErrCommandBlocked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &recordingSDKClient{}
			client := NewClient(raw)

			err := client.RecoverEffect(context.Background(), tt.command)

			require.ErrorIs(t, err, tt.wantErr)
			require.Empty(t, raw.workflowName)
			require.Empty(t, raw.startOptions.ID)
		})
	}
}

func TestTemporalClientRecoverEffectAcceptsSecondaryRecoverableEffectOwner(t *testing.T) {
	raw := &recordingSDKClient{}
	client := NewClient(raw)
	projection := imageagent.RunProjection{
		Run: imageagent.Run{
			ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual,
			Status: imageagent.RunStatusBlocked, Block: &imageagent.Block{Code: "recovery_requested", SlotID: "slot-1"},
		},
		Plan: imageagent.Plan{
			Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1", "source-2"},
			Slots: []imageagent.Slot{
				{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusPending},
				{ID: "slot-2", Role: imageagent.SlotRoleScene, SourceAssetIDs: []string{"source-2"}, IdempotencyKey: "slot-key-2", Status: imageagent.SlotStatusPending},
			},
		},
		Slots: []imageagent.SlotProjection{
			{Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusBlocked}, Attempt: 1, ErrorCode: "recovery_requested"},
			{Slot: imageagent.Slot{ID: "slot-2", Role: imageagent.SlotRoleScene, SourceAssetIDs: []string{"source-2"}, IdempotencyKey: "slot-key-2", Status: imageagent.SlotStatusBlocked}, Attempt: 2, ErrorCode: "recovery_start_failed"},
		},
		RecoverableEffects: []imageagent.RecoverableEffect{
			{SlotID: "slot-1", Attempt: 1, Code: "recovery_requested"},
			{SlotID: "slot-2", Attempt: 2, Code: "recovery_start_failed"},
		},
		AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-1.png"},
			{ID: "source-2", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-2.png"},
		}},
	}

	err := client.RecoverEffect(context.Background(), imageagent.RecoverEffectCommand{
		RunID: "run-1", PlanRevision: 1, SlotID: "slot-2", Attempt: 2, ActionID: "recover-2",
		Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, Projection: projection,
	})

	require.NoError(t, err)
	require.Equal(t, EffectRecoveryWorkflowName, raw.workflowName)
	require.Equal(t, projection.AssetCatalog, raw.effectRecoveryInput.AssetCatalog)
	require.Equal(t, projection.Plan.Slots[1], raw.effectRecoveryInput.Slot)
	require.Equal(t, 2, raw.effectRecoveryInput.Attempt)
}
