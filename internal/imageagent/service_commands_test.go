package imageagent_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func TestServiceApprovalRequiresExactCurrentProjectionDigestAndState(t *testing.T) {
	tests := []struct {
		name       string
		status     imageagent.RunStatus
		digest     string
		wantErr    error
		wantSignal bool
	}{
		{name: "exact digest", status: imageagent.RunStatusAwaitingFinalApproval, digest: "digest-1", wantSignal: true},
		{name: "missing digest", status: imageagent.RunStatusAwaitingFinalApproval, digest: "", wantErr: imageagent.ErrCommandBlocked},
		{name: "mismatched digest", status: imageagent.RunStatusAwaitingFinalApproval, digest: "other", wantErr: imageagent.ErrCommandBlocked},
		{name: "out of state", status: imageagent.RunStatusExecuting, digest: "digest-1", wantErr: imageagent.ErrCommandBlocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := seededRepository(t, imageagent.RunStatusAwaitingFinalApproval, nil)
			workflows := &recordingWorkflowClient{projection: imageagent.WorkflowProjection{
				Status: tt.status, Plan: commandPlan(1), ResultDigest: "digest-1",
			}}
			service, err := imageagent.NewService(repository, workflows)
			require.NoError(t, err)

			err = service.ApproveResults(verifiedContext("tenant-a", "user-a"), "run-1", 1, tt.digest, "approve-1")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantSignal, len(workflows.approvals) == 1)
		})
	}
}

func TestServiceReplacePlanIsBlockedUnlessCurrentRunIsBlockedAtExpectedRevision(t *testing.T) {
	tests := []struct {
		name       string
		status     imageagent.RunStatus
		expected   int64
		wantErr    error
		wantSignal bool
	}{
		{name: "blocked current revision", status: imageagent.RunStatusBlocked, expected: 1, wantSignal: true},
		{name: "wrong state", status: imageagent.RunStatusExecuting, expected: 1, wantErr: imageagent.ErrCommandBlocked},
		{name: "stale revision", status: imageagent.RunStatusBlocked, expected: 0, wantErr: imageagent.ErrRevisionConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := seededRepository(t, tt.status, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
			workflows := &recordingWorkflowClient{projection: imageagent.WorkflowProjection{Status: tt.status, Plan: commandPlan(1)}}
			service, err := imageagent.NewService(repository, workflows)
			require.NoError(t, err)

			replacement := commandPlan(2)
			replacement.ParentRevision = 1
			err = service.ReplacePlan(verifiedContext("tenant-a", "user-a"), "run-1", tt.expected, replacement, "replace-1")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, "user-a", workflows.replacements[0].ActorID)
			}
			require.Equal(t, tt.wantSignal, len(workflows.replacements) == 1)
		})
	}
}

func TestServiceRetryAndCancelRejectBlockedOrTerminalCommands(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
	workflows := &recordingWorkflowClient{projection: imageagent.WorkflowProjection{Status: imageagent.RunStatusBlocked, Plan: commandPlan(1), Block: &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"}}}
	service, err := imageagent.NewService(repository, workflows)
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-a")

	require.ErrorIs(t, service.RetrySlot(ctx, "run-1", "slot-2", 1, "retry-wrong"), imageagent.ErrCommandBlocked)
	require.NoError(t, service.RetrySlot(ctx, "run-1", "slot-1", 1, "retry-1"))
	require.Len(t, workflows.retries, 1)

	terminalRepository := seededRepository(t, imageagent.RunStatusCompleted, nil)
	terminalWorkflows := &recordingWorkflowClient{projection: imageagent.WorkflowProjection{Status: imageagent.RunStatusCompleted, Plan: commandPlan(1)}}
	terminalService, err := imageagent.NewService(terminalRepository, terminalWorkflows)
	require.NoError(t, err)
	require.ErrorIs(t, terminalService.Cancel(ctx, "run-1", 1, "cancel-1"), imageagent.ErrCommandBlocked)
	require.Empty(t, terminalWorkflows.cancellations)
}

func TestServiceGetProjectionUsesVerifiedTenantAndDurableCursor(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
	require.NoError(t, repository.AppendEvent(context.Background(), imageagent.RunEvent{
		TenantID: "tenant-a", RunID: "run-1", Type: "slot.result.persisted", Cursor: 2,
		ProjectionVersion: 2, Payload: json.RawMessage(`{"slot_id":"slot-1"}`),
	}))
	workflows := &recordingWorkflowClient{projection: imageagent.WorkflowProjection{
		Status: imageagent.RunStatusBlocked, Plan: commandPlan(1),
		Block: &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"},
		Slots: []imageagent.SlotProjection{{Slot: commandPlan(1).Slots[0], Attempt: 1, ErrorCode: "provider_failed"}},
	}}
	service, err := imageagent.NewService(repository, workflows)
	require.NoError(t, err)

	projection, err := service.Get(verifiedContext("tenant-a", "user-a"), "run-1")
	require.NoError(t, err)
	require.EqualValues(t, 2, projection.LastEventID)
	require.Equal(t, imageagent.RunStatusBlocked, projection.Run.Status)
	require.Equal(t, commandPlan(1), projection.Plan)
	require.Equal(t, []imageagent.Action{imageagent.ActionEditPlan, imageagent.ActionRetrySlot, imageagent.ActionCancel}, projection.Actions)

	_, err = service.Get(verifiedContext("tenant-b", "user-a"), "run-1")
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
}

func seededRepository(t *testing.T, status imageagent.RunStatus, block *imageagent.Block) imageagent.Repository {
	t.Helper()
	repository := store.NewMemoryRepository()
	run := &imageagent.Run{
		ID: "run-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-key-1", Status: imageagent.RunStatusPlanning, CurrentNode: "plan", Version: 1,
	}
	require.NoError(t, repository.CreateRun(context.Background(), run))
	require.NoError(t, repository.AppendPlan(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, 0, commandPlan(1)))
	if status != imageagent.RunStatusPlanning || block != nil {
		require.NoError(t, repository.UpdateRun(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, 1, imageagent.RunMutation{
			Status: status, CurrentNode: "command", ActivePlanRevision: 1, Block: block,
		}))
	}
	return repository
}

func commandPlan(revision int64) imageagent.Plan {
	return imageagent.Plan{
		Revision: revision, IdempotencyKey: "plan-key-" + string(rune('0'+revision)), SourceAssetIDs: []string{"source-1"},
		Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleScene, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1"}},
	}
}

func verifiedContext(tenantID, userID string) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: tenantID, UserID: userID})
}

type recordingWorkflowClient struct {
	projection    imageagent.WorkflowProjection
	projectionErr error
	replacements  []imageagent.ReplacePlanCommand
	retries       []imageagent.RetrySlotCommand
	approvals     []imageagent.ApproveResultsCommand
	cancellations []imageagent.CancelRunCommand
}

func (*recordingWorkflowClient) StartManual(context.Context, imageagent.WorkflowStart) error {
	return nil
}

func (c *recordingWorkflowClient) GetProjection(context.Context, imageagent.RunScope, imageagent.ExecutionIdentity) (imageagent.WorkflowProjection, error) {
	return c.projection, c.projectionErr
}

func (c *recordingWorkflowClient) ReplacePlan(_ context.Context, command imageagent.ReplacePlanCommand) error {
	c.replacements = append(c.replacements, command)
	return nil
}

func (c *recordingWorkflowClient) RetrySlot(_ context.Context, command imageagent.RetrySlotCommand) error {
	c.retries = append(c.retries, command)
	return nil
}

func (c *recordingWorkflowClient) ApproveResults(_ context.Context, command imageagent.ApproveResultsCommand) error {
	c.approvals = append(c.approvals, command)
	return nil
}

func (c *recordingWorkflowClient) Cancel(_ context.Context, command imageagent.CancelRunCommand) error {
	c.cancellations = append(c.cancellations, command)
	return nil
}
