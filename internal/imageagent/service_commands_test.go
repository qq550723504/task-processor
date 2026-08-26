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

func TestServiceGetProjectionUsesVerifiedOwnerAndOnlyDurableSnapshot(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
	workflows := &recordingWorkflowClient{projection: imageagent.WorkflowProjection{
		Status: imageagent.RunStatusBlocked, Plan: commandPlan(1),
		Block: &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"},
		Slots: []imageagent.SlotProjection{{Slot: commandPlan(1).Slots[0], Attempt: 1, ErrorCode: "provider_failed"}},
	}}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)

	projection, err := service.Get(verifiedContext("tenant-a", "user-a"), "run-1")
	require.NoError(t, err)
	require.EqualValues(t, 1, projection.LastEventID)
	require.Equal(t, imageagent.RunStatusBlocked, projection.Run.Status)
	require.Equal(t, commandPlan(1), projection.Plan)
	require.Equal(t, []imageagent.Action{imageagent.ActionEditPlan, imageagent.ActionRetrySlot, imageagent.ActionCancel}, projection.Actions)
	require.Zero(t, workflows.projectionCalls, "public GET must not query Temporal")

	_, err = service.Get(verifiedContext("tenant-b", "user-a"), "run-1")
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = service.Get(verifiedContext("tenant-a", "user-b"), "run-1")
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
}

func TestServiceSameTenantDifferentOwnerCannotReadOrCommandRun(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-b")

	_, err = service.Get(ctx, "run-1")
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = service.ListEvents(ctx, "run-1", 0, 10)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = service.Resume(ctx, "run-1", "pending-action")
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	replacement := commandPlan(2)
	replacement.ParentRevision = 1
	require.ErrorIs(t, service.ReplacePlan(ctx, "run-1", 1, replacement, "replace-owner-b"), imageagent.ErrRunNotFound)
	require.ErrorIs(t, service.RetrySlot(ctx, "run-1", "slot-1", 1, "retry-owner-b"), imageagent.ErrRunNotFound)
	require.ErrorIs(t, service.ApproveResults(ctx, "run-1", 1, "digest-1", "approve-owner-b"), imageagent.ErrRunNotFound)
	require.ErrorIs(t, service.Cancel(ctx, "run-1", 1, "cancel-owner-b"), imageagent.ErrRunNotFound)
	require.Empty(t, workflows.replacements)
	require.Empty(t, workflows.retries)
	require.Empty(t, workflows.approvals)
	require.Empty(t, workflows.cancellations)
	require.Empty(t, workflows.resumes)
}

func seededRepository(t *testing.T, status imageagent.RunStatus, block *imageagent.Block) imageagent.Repository {
	t.Helper()
	repository := store.NewMemoryRepository()
	run := &imageagent.Run{
		ID: "run-1", BusinessTaskID: "task-1", TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-key-1", Status: status, CurrentNode: "command", Version: 1, Block: block, ActivePlanRevision: 1,
	}
	plan := commandPlan(1)
	_, err := repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: imageagent.ScopeForRun(*run), Run: *run, Plan: plan, Catalog: authorizedCatalog(), Snapshot: imageagent.RunProjection{Run: *run, Plan: plan, Slots: []imageagent.SlotProjection{{Slot: plan.Slots[0], ErrorCode: func() string {
		if block != nil {
			return "provider_failed"
		}
		return ""
	}()}}, ResultDigest: "digest-1"}, CommitID: "seed", EventType: "run.initialized", EventPayload: json.RawMessage(`{}`)})
	require.NoError(t, err)
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
	projection      imageagent.WorkflowProjection
	projectionErr   error
	projectionCalls int
	starts          []imageagent.WorkflowStart
	replacements    []imageagent.ReplacePlanCommand
	retries         []imageagent.RetrySlotCommand
	approvals       []imageagent.ApproveResultsCommand
	cancellations   []imageagent.CancelRunCommand
	replaceErr      error
	retryErr        error
	approveErr      error
	cancelErr       error
	resumes         []imageagent.ResumeCommand
	resumeAck       imageagent.CommandAcknowledgement
	resumeErr       error
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

func TestServiceStartRetryUsesImmutablePersistedCatalogInsteadOfMutableTaskCatalog(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	resolver := &mutableCatalogResolver{catalog: authorizedCatalog()}
	service, err := imageagent.NewService(repository, workflows, resolver)
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-a")
	input := imageagent.StartRunInput{RunID: "run-start-retry", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual, IdempotencyKey: "run-start-retry-key", Plan: commandPlan(1)}

	require.NoError(t, service.Start(ctx, input))
	resolver.catalog = imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-replaced", Type: imageagent.AuthorizedAssetSource, URL: "https://mutated.example/source.png"}}}
	require.NoError(t, service.Start(ctx, input))
	require.Equal(t, 1, resolver.calls, "idempotent Start must not re-read mutable business-task assets")
	require.Len(t, workflows.starts, 2)
	require.Equal(t, workflows.starts[0].AssetCatalog, workflows.starts[1].AssetCatalog)
	require.Equal(t, "source-1", workflows.starts[1].AssetCatalog.Assets[0].ID)
}

func TestServiceReplacePlanValidatesPlanAndSlotAssetsAgainstPersistedRunCatalog(t *testing.T) {
	repository := seededRepository(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})
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

type mutableCatalogResolver struct {
	catalog imageagent.AssetCatalog
	calls   int
}

func (r *mutableCatalogResolver) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	r.calls++
	return r.catalog, nil
}

func (c *recordingWorkflowClient) StartManual(_ context.Context, start imageagent.WorkflowStart) error {
	c.starts = append(c.starts, start)
	return nil
}

func (c *recordingWorkflowClient) GetProjection(context.Context, imageagent.RunScope, imageagent.ExecutionIdentity) (imageagent.WorkflowProjection, error) {
	c.projectionCalls++
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
