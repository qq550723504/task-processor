package imageagent_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
	"task-processor/internal/shared/aiidentity"
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
		Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1", Status: imageagent.SlotStatusPending}},
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
	startErr        error
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

func TestServiceStartRejectsRunIDOutsideArtifactGrammarBeforeWorkflowStart(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)

	err = service.Start(verifiedContext("tenant-a", "user-a"), imageagent.StartRunInput{
		RunID: "run:1", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-start-key", Plan: commandPlan(1),
	})

	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.Empty(t, workflows.starts)
}

func TestServiceStartRejectsUnsafeSlotConcurrencyBeforePersistence(t *testing.T) {
	for _, maxConcurrentSlots := range []int{-1, 11} {
		repository := store.NewMemoryRepository()
		workflows := &recordingWorkflowClient{}
		service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
		require.NoError(t, err)

		err = service.Start(verifiedContext("tenant-a", "user-a"), imageagent.StartRunInput{
			RunID: "run-concurrency", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual,
			IdempotencyKey: "run-concurrency-key", Plan: commandPlan(1), MaxConcurrentSlots: maxConcurrentSlots,
		})

		require.ErrorIs(t, err, imageagent.ErrValidation)
		_, getErr := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-concurrency"})
		require.ErrorIs(t, getErr, imageagent.ErrRunNotFound)
		require.Empty(t, workflows.starts)
	}
}

func TestServiceStartCapturesBusinessTaskInExecutionIdentity(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TraceID: "trace-identity"})
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	err = service.Start(ctx, imageagent.StartRunInput{
		RunID: "run-identity", BusinessTaskID: "task-identity", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-identity-key", Plan: commandPlan(1),
	})

	require.NoError(t, err)
	require.Len(t, workflows.starts, 1)
	require.Equal(t, "task-identity", workflows.starts[0].Identity.BusinessTaskID)
	require.Equal(t, "trace-identity", workflows.starts[0].Identity.TraceID)
}

func TestServiceStartValidatesAndPersistsPresenceAwareBudget(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	input := imageagent.StartRunInput{
		RunID: "run-budget", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-budget-key", Plan: commandPlan(1),
		Budget: imageagent.Budget{EnabledLimits: imageagent.BudgetLimitImages},
	}

	require.NoError(t, service.Start(verifiedContext("tenant-a", "user-a"), input))
	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-budget"})
	require.NoError(t, err)
	policy, err := projection.Run.Budget.Policy()
	require.NoError(t, err)
	require.Equal(t, imageagent.Limit{Enabled: true, Value: 0}, policy.Images)
	require.Len(t, workflows.starts, 1)

	invalid := input
	invalid.RunID = "run-budget-invalid"
	invalid.IdempotencyKey = "run-budget-invalid-key"
	invalid.Budget = imageagent.Budget{MaxImages: -1}
	require.ErrorIs(t, service.Start(verifiedContext("tenant-a", "user-a"), invalid), imageagent.ErrValidation)
	require.Len(t, workflows.starts, 1)
}

func TestServiceRetryRejectsSlotIDOutsideArtifactGrammarBeforeWorkflowUpdate(t *testing.T) {
	service, workflows := commandService(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})

	err := service.RetrySlot(verifiedContext("tenant-a", "user-a"), "run-1", "slot/1", 1, "retry-invalid-slot")

	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.Empty(t, workflows.retries)
}

func TestServiceRetryCarriesPersistedBusinessTaskIdentity(t *testing.T) {
	service, workflows := commandService(t, imageagent.RunStatusBlocked, &imageagent.Block{Code: "slot_failed", SlotID: "slot-1"})

	err := service.RetrySlot(verifiedContext("tenant-a", "user-a"), "run-1", "slot-1", 1, "retry-identity")

	require.NoError(t, err)
	require.Len(t, workflows.retries, 1)
	require.Equal(t, "task-1", workflows.retries[0].Identity.BusinessTaskID)
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

func TestServiceCompletedStartReplayReturnsOriginalSuccessWithoutRestartingTemporal(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: authorizedCatalog()})
	require.NoError(t, err)
	input := imageagent.StartRunInput{
		RunID: "run-1", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-key-1", Plan: commandPlan(1),
	}
	ctx := verifiedContext("tenant-a", "user-a")
	require.NoError(t, service.Start(ctx, input))
	current, err := repository.GetProjection(ctx, imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: "run-1"})
	require.NoError(t, err)
	completed := current
	completed.Run.Status = imageagent.RunStatusCompleted
	completed.Run.CurrentNode = "complete"
	completed.Run.Version++
	_, err = repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: imageagent.ScopeForRun(current.Run), CommitID: "complete", ExpectedProjectionVersion: current.ProjectionVersion,
		Snapshot: completed, EventType: "run.completed", EventPayload: json.RawMessage(`{}`), ExpectedRunVersion: current.Run.Version,
		RunMutation: &imageagent.RunMutation{Status: imageagent.RunStatusCompleted, CurrentNode: "complete", ActivePlanRevision: current.Run.ActivePlanRevision},
	})
	require.NoError(t, err)
	workflows.starts = nil
	workflows.startErr = errors.New("completed workflow cannot be started again")

	err = service.Start(ctx, input)

	require.NoError(t, err)
	require.Empty(t, workflows.starts)
}

func TestServiceConcurrentIdenticalStartWithRepositoryOwnedCatalogTimestampConverges(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &concurrentStartWorkflowClient{}
	resolver := newConcurrentCatalogResolver(authorizedCatalog(), 2)
	service, err := imageagent.NewService(repository, workflows, resolver)
	require.NoError(t, err)
	input := imageagent.StartRunInput{RunID: "run-concurrent-start", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual, IdempotencyKey: "run-concurrent-start-key", Plan: commandPlan(1), MaxConcurrentSlots: 0}

	errs := make(chan error, 2)
	for range 2 {
		go func() {
			errs <- service.Start(verifiedContext("tenant-a", "user-a"), input)
		}()
	}
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, 2, resolver.Calls())
	require.Equal(t, 2, workflows.Count())
	for _, start := range workflows.Starts() {
		require.Equal(t, 4, start.MaxConcurrentSlots)
	}

	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: input.RunID})
	require.NoError(t, err)
	require.False(t, projection.AssetCatalog.Manifest.CreatedAt.IsZero(), "repository winner must assign the durable catalog creation time")
	require.Equal(t, 4, projection.Run.MaxConcurrentSlots)
}

func TestServiceStartUsesRepositoryWinnerConcurrencyForReplayAndRejectsConflictingUseExisting(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	resolver := &mutableCatalogResolver{catalog: authorizedCatalog()}
	service, err := imageagent.NewService(repository, workflows, resolver)
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-a")
	input := imageagent.StartRunInput{RunID: "run-start-concurrency", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual, IdempotencyKey: "run-start-concurrency-key", Plan: commandPlan(1), MaxConcurrentSlots: 2}

	require.NoError(t, service.Start(ctx, input))
	require.NoError(t, service.Start(ctx, input))
	require.Len(t, workflows.starts, 2)
	require.Equal(t, 2, workflows.starts[0].MaxConcurrentSlots)
	require.Equal(t, 2, workflows.starts[1].MaxConcurrentSlots)

	conflict := input
	conflict.MaxConcurrentSlots = 3
	require.ErrorIs(t, service.Start(ctx, conflict), imageagent.ErrRevisionConflict)
	require.Len(t, workflows.starts, 2, "conflicting USE_EXISTING must not start or signal Temporal")

	projection, err := repository.GetProjection(ctx, imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: input.RunID})
	require.NoError(t, err)
	require.Equal(t, 2, projection.Run.MaxConcurrentSlots)
}

func TestServiceConcurrentConflictingStartConcurrencyKeepsRepositoryWinnerValue(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &concurrentStartWorkflowClient{}
	resolver := newConcurrentCatalogResolver(authorizedCatalog(), 2)
	service, err := imageagent.NewService(repository, workflows, resolver)
	require.NoError(t, err)
	base := imageagent.StartRunInput{RunID: "run-concurrent-conflict", BusinessTaskID: "task-1", Mode: imageagent.RunModeManual, IdempotencyKey: "run-concurrent-conflict-key", Plan: commandPlan(1)}
	inputs := []imageagent.StartRunInput{base, base}
	inputs[0].MaxConcurrentSlots = 2
	inputs[1].MaxConcurrentSlots = 3

	errs := make(chan error, 2)
	for _, input := range inputs {
		input := input
		go func() { errs <- service.Start(verifiedContext("tenant-a", "user-a"), input) }()
	}
	first, second := <-errs, <-errs
	require.True(t, (first == nil && errors.Is(second, imageagent.ErrRevisionConflict)) || (second == nil && errors.Is(first, imageagent.ErrRevisionConflict)), "results = %v, %v", first, second)
	require.Equal(t, 1, workflows.Count())

	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: base.RunID})
	require.NoError(t, err)
	starts := workflows.Starts()
	require.Len(t, starts, 1)
	require.Equal(t, projection.Run.MaxConcurrentSlots, starts[0].MaxConcurrentSlots, "Temporal must use repository winner value")
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

type concurrentCatalogResolver struct {
	catalog imageagent.AssetCatalog
	ready   sync.WaitGroup
	release chan struct{}
	mu      sync.Mutex
	calls   int
}

func newConcurrentCatalogResolver(catalog imageagent.AssetCatalog, callers int) *concurrentCatalogResolver {
	resolver := &concurrentCatalogResolver{catalog: catalog, release: make(chan struct{})}
	resolver.ready.Add(callers)
	go func() {
		resolver.ready.Wait()
		close(resolver.release)
	}()
	return resolver
}

func (r *concurrentCatalogResolver) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	r.mu.Lock()
	r.calls++
	call := r.calls
	r.mu.Unlock()
	r.ready.Done()
	<-r.release
	if call == 2 {
		time.Sleep(20 * time.Millisecond)
	}
	return r.catalog, nil
}

func (r *concurrentCatalogResolver) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type concurrentStartWorkflowClient struct {
	recordingWorkflowClient
	mu     sync.Mutex
	starts []imageagent.WorkflowStart
}

func (c *concurrentStartWorkflowClient) StartManual(_ context.Context, start imageagent.WorkflowStart) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts = append(c.starts, start)
	return nil
}

func (c *concurrentStartWorkflowClient) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.starts)
}

func (c *concurrentStartWorkflowClient) Starts() []imageagent.WorkflowStart {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]imageagent.WorkflowStart(nil), c.starts...)
}

func (r *mutableCatalogResolver) Resolve(context.Context, imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	r.calls++
	return r.catalog, nil
}

func (c *recordingWorkflowClient) StartManual(_ context.Context, start imageagent.WorkflowStart) error {
	c.starts = append(c.starts, start)
	return c.startErr
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
