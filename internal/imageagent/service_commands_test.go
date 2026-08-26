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
		name        string
		digest      string
		workflowErr error
		wantErr     error
		wantUpdate  bool
	}{
		{name: "exact digest", digest: "digest-1", wantUpdate: true},
		{name: "missing digest", digest: "", wantErr: imageagent.ErrCommandBlocked},
		{name: "mismatched digest", digest: "other", workflowErr: imageagent.ErrCommandBlocked, wantErr: imageagent.ErrCommandBlocked, wantUpdate: true},
		{name: "out of state", digest: "digest-1", workflowErr: imageagent.ErrCommandBlocked, wantErr: imageagent.ErrCommandBlocked, wantUpdate: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := seededRepository(t, imageagent.RunStatusAwaitingFinalApproval, nil)
			workflows := &recordingWorkflowClient{approveErr: tt.workflowErr}
			service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
			require.NoError(t, err)

			err = service.ApproveResults(verifiedContext("tenant-a", "user-a"), "run-1", 1, tt.digest, "approve-1")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantUpdate, len(workflows.approvals) == 1)
		})
	}
}

func TestServiceApprovalRejectsNonCanonicalDigestBeforeWorkflowUpdate(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusAwaitingFinalApproval, nil)
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)

	err = service.ApproveResults(verifiedContext("tenant-a", "user-a"), "run-1", 1, " digest-1 ", "approve-1")

	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Empty(t, workflows.approvals)
}

func TestServiceDefersCommandRevisionAndStateAcceptanceToWorkflowUpdate(t *testing.T) {
	ctx := verifiedContext("tenant-a", "user-a")
	t.Run("replace", func(t *testing.T) {
		service, workflows := commandService(t, imageagent.RunStatusExecuting, nil)
		workflows.replaceErr = imageagent.ErrCommandBlocked
		replacement := commandPlan(2)
		replacement.ParentRevision = 1
		err := service.ReplacePlan(ctx, "run-1", 1, replacement, "replace-1")
		require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
		require.Len(t, workflows.replacements, 1)
	})
	t.Run("stale retry", func(t *testing.T) {
		service, workflows := commandService(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
		workflows.retryErr = imageagent.ErrRevisionConflict
		err := service.RetrySlot(ctx, "run-1", "slot-1", 2, "retry-1")
		require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
		require.Len(t, workflows.retries, 1)
	})
	t.Run("approval state", func(t *testing.T) {
		service, workflows := commandService(t, imageagent.RunStatusExecuting, nil)
		workflows.approveErr = imageagent.ErrCommandBlocked
		err := service.ApproveResults(ctx, "run-1", 1, "digest-1", "approve-1")
		require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
		require.Len(t, workflows.approvals, 1)
	})
	t.Run("terminal cancel", func(t *testing.T) {
		service, workflows := commandService(t, imageagent.RunStatusCompleted, nil)
		workflows.cancelErr = imageagent.ErrCommandBlocked
		err := service.Cancel(ctx, "run-1", 1, "cancel-1")
		require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
		require.Len(t, workflows.cancellations, 1)
	})
}

func TestServiceReplacePlanIsBlockedUnlessCurrentRunIsBlockedAtExpectedRevision(t *testing.T) {
	tests := []struct {
		name         string
		status       imageagent.RunStatus
		expected     int64
		planRevision int64
		workflowErr  error
		wantErr      error
		wantUpdate   bool
	}{
		{name: "blocked current revision", status: imageagent.RunStatusBlocked, expected: 1, planRevision: 2, wantUpdate: true},
		{name: "wrong state", status: imageagent.RunStatusExecuting, expected: 1, planRevision: 2, workflowErr: imageagent.ErrCommandBlocked, wantErr: imageagent.ErrCommandBlocked, wantUpdate: true},
		{name: "stale revision", status: imageagent.RunStatusBlocked, expected: 2, planRevision: 3, workflowErr: imageagent.ErrRevisionConflict, wantErr: imageagent.ErrRevisionConflict, wantUpdate: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := seededRepository(t, tt.status, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
			workflows := &recordingWorkflowClient{replaceErr: tt.workflowErr}
			service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
			require.NoError(t, err)

			replacement := commandPlan(tt.planRevision)
			replacement.ParentRevision = tt.expected
			err = service.ReplacePlan(verifiedContext("tenant-a", "user-a"), "run-1", tt.expected, replacement, "replace-1")
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				require.Equal(t, "user-a", workflows.replacements[0].ActorID)
			}
			require.Equal(t, tt.wantUpdate, len(workflows.replacements) == 1)
		})
	}
}

func TestServiceRetryAndCancelRejectBlockedOrTerminalCommands(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
	workflows := &recordingWorkflowClient{retryErr: imageagent.ErrCommandBlocked}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-a")

	require.ErrorIs(t, service.RetrySlot(ctx, "run-1", "slot-2", 1, "retry-wrong"), imageagent.ErrCommandBlocked)
	require.Len(t, workflows.retries, 1)
	workflows.retryErr = nil
	require.NoError(t, service.RetrySlot(ctx, "run-1", "slot-1", 1, "retry-1"))
	require.Len(t, workflows.retries, 2)

	terminalRepository := seededRepository(t, imageagent.RunStatusCompleted, nil)
	terminalWorkflows := &recordingWorkflowClient{cancelErr: imageagent.ErrCommandBlocked}
	terminalService, err := imageagent.NewService(terminalRepository, terminalWorkflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	require.ErrorIs(t, terminalService.Cancel(ctx, "run-1", 1, "cancel-1"), imageagent.ErrCommandBlocked)
	require.Len(t, terminalWorkflows.cancellations, 1)
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
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
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
		ID: "run-1", BusinessTaskID: "task-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-key-1", Status: imageagent.RunStatusPlanning, CurrentNode: "plan", Version: 1,
	}
	require.NoError(t, repository.CreateRun(context.Background(), run))
	require.NoError(t, repository.SaveAssetCatalog(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, authorizedCatalog()))
	require.NoError(t, repository.AppendPlan(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, 0, commandPlan(1)))
	if status != imageagent.RunStatusPlanning || block != nil {
		require.NoError(t, repository.UpdateRun(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, 1, imageagent.RunMutation{
			Status: status, CurrentNode: "command", ActivePlanRevision: 1, Block: block,
		}))
	}
	return repository
}

func commandService(t *testing.T, status imageagent.RunStatus, block *imageagent.Block) (*imageagent.Service, *recordingWorkflowClient) {
	t.Helper()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(seededRepository(t, status, block), workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	return service, workflows
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
	starts        []imageagent.WorkflowStart
	replacements  []imageagent.ReplacePlanCommand
	retries       []imageagent.RetrySlotCommand
	approvals     []imageagent.ApproveResultsCommand
	cancellations []imageagent.CancelRunCommand
	replaceErr    error
	retryErr      error
	approveErr    error
	cancelErr     error
	resumes       []imageagent.ResumeCommand
	resumeAck     imageagent.CommandAcknowledgement
	resumeErr     error
}

func TestServiceStartRequiresBusinessTaskAndAuthorizedCatalogSubset(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-a")
	input := imageagent.StartRunInput{RunID: "run-start", BusinessTaskID: " ", Mode: imageagent.RunModeManual, IdempotencyKey: "run-start-key", Plan: commandPlan(1)}

	require.ErrorIs(t, service.Start(ctx, input), imageagent.ErrValidation)
	input.BusinessTaskID = "task-1"
	input.Plan.SourceAssetIDs = []string{"source-not-authorized"}
	input.Plan.Slots[0].SourceAssetIDs = []string{"source-not-authorized"}
	require.ErrorIs(t, service.Start(ctx, input), imageagent.ErrValidation)
	require.Empty(t, workflows.starts)
}

func TestServiceReplacePlanValidatesPlanAndSlotAssetsAgainstPersistedRunCatalog(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
	require.NoError(t, repository.SaveAssetCatalog(context.Background(), imageagent.RunScope{TenantID: "tenant-a", RunID: "run-1"}, authorizedCatalog()))
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	replacement := commandPlan(2)
	replacement.ParentRevision = 1
	replacement.StyleReferenceIDs = []string{"style-1"}
	replacement.Slots[0].StyleReferenceIDs = []string{"style-1"}
	require.NoError(t, service.ReplacePlan(verifiedContext("tenant-a", "user-a"), "run-1", 1, replacement, "replace-authorized"))

	replacement.Revision = 3
	replacement.ParentRevision = 1
	replacement.IdempotencyKey = "plan-key-3"
	replacement.Slots[0].SourceAssetIDs = []string{"candidate-generated"}
	require.ErrorIs(t, service.ReplacePlan(verifiedContext("tenant-a", "user-a"), "run-1", 1, replacement, "replace-injected"), imageagent.ErrValidation)
	require.Len(t, workflows.replacements, 1)
}

func authorizedCatalog() imageagent.AssetCatalog {
	return imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
		{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-1.png", Label: "Source 1"},
		{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, DisplayURL: "https://cdn.example.test/style-1.png", Label: "Style 1"},
	}}
}

type staticCatalogResolver struct{ catalog imageagent.AssetCatalog }

func (r staticCatalogResolver) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	return r.catalog, nil
}

func (c *recordingWorkflowClient) StartManual(_ context.Context, start imageagent.WorkflowStart) error {
	c.starts = append(c.starts, start)
	return nil
}

func (c *recordingWorkflowClient) GetProjection(context.Context, imageagent.RunScope, imageagent.ExecutionIdentity) (imageagent.WorkflowProjection, error) {
	return c.projection, c.projectionErr
}

func (c *recordingWorkflowClient) ReplacePlan(_ context.Context, command imageagent.ReplacePlanCommand) error {
	c.replacements = append(c.replacements, command)
	return c.replaceErr
}

func (c *recordingWorkflowClient) RetrySlot(_ context.Context, command imageagent.RetrySlotCommand) error {
	c.retries = append(c.retries, command)
	return c.retryErr
}

func (c *recordingWorkflowClient) ApproveResults(_ context.Context, command imageagent.ApproveResultsCommand) error {
	c.approvals = append(c.approvals, command)
	return c.approveErr
}

func (c *recordingWorkflowClient) Cancel(_ context.Context, command imageagent.CancelRunCommand) error {
	c.cancellations = append(c.cancellations, command)
	return c.cancelErr
}

func (c *recordingWorkflowClient) Resume(_ context.Context, command imageagent.ResumeCommand) (imageagent.CommandAcknowledgement, error) {
	c.resumes = append(c.resumes, command)
	return c.resumeAck, c.resumeErr
}

func TestServiceResumeUsesVerifiedActorAndReturnsWorkflowAcknowledgement(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
	want := imageagent.CommandAcknowledgement{RunID: "run-1", PlanRevision: 1, ActionID: "retry-pending", Status: imageagent.RunStatusAwaitingFinalApproval}
	workflows := &recordingWorkflowClient{resumeAck: want}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)

	got, err := service.Resume(verifiedContext("tenant-a", "user-a"), "run-1", "retry-pending")
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, "user-a", workflows.resumes[0].ActorID)
	require.Equal(t, imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, workflows.resumes[0].Identity)
	_, err = service.Resume(verifiedContext("tenant-b", "user-a"), "run-1", "retry-pending")
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
}
