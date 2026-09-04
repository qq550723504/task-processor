package imageagent_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func launchCatalog() imageagent.AssetCatalog {
	return imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
		{ID: "source-1", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-1.png", Label: "Source 1", Width: 1200, Height: 900},
		{ID: "source-2", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source-2.png", Label: "Source 2", Width: 1200, Height: 900},
		{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, DisplayURL: "https://cdn.example.test/style-1.png", Label: "Style 1"},
	}}
}

// taskScopedCatalogResolver mirrors the composition-owned catalog selection:
// a selected primary source narrows sources, and an explicit style selection
// narrows styles — while an absent style selection preserves every task-owned
// style in the run-scoped authorization snapshot.
type taskScopedCatalogResolver struct{ catalog imageagent.AssetCatalog }

func (r taskScopedCatalogResolver) Resolve(_ context.Context, scope imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	styleSelectionProvided := len(scope.StyleReferenceIDs) > 0
	selectedStyles := make(map[string]struct{}, len(scope.StyleReferenceIDs))
	for _, id := range scope.StyleReferenceIDs {
		selectedStyles[id] = struct{}{}
	}
	assets := make([]imageagent.AuthorizedAsset, 0, len(r.catalog.Assets))
	for _, asset := range r.catalog.Assets {
		if asset.Type == imageagent.AuthorizedAssetSource {
			if scope.PrimarySourceAssetID == "" || asset.ID == scope.PrimarySourceAssetID {
				assets = append(assets, asset)
			}
			continue
		}
		if !styleSelectionProvided {
			assets = append(assets, asset)
			continue
		}
		if _, ok := selectedStyles[asset.ID]; ok {
			assets = append(assets, asset)
		}
	}
	return imageagent.AssetCatalog{Assets: assets}, nil
}

func launchTaskRunInput() imageagent.TaskRunLaunchInput {
	return imageagent.TaskRunLaunchInput{
		RequestID:      "launch-request-1",
		BusinessTaskID: "task-1", TargetPlatform: "shein",
		ImagePolicyContext: imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"},
		SourceAssetID:      "source-1",
	}
}

func TestServiceLaunchTaskRunStartsSingleMainSlotRunWithExplicitSource(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, taskScopedCatalogResolver{catalog: launchCatalog()})
	require.NoError(t, err)

	result, err := service.LaunchTaskRun(verifiedContext("tenant-a", "user-a"), launchTaskRunInput())

	require.NoError(t, err)
	require.NotEmpty(t, result.RunID)
	require.NoError(t, imageagent.ValidateArtifactKeyIdentifier(result.RunID))
	require.Len(t, workflows.starts, 1)

	start := workflows.starts[0]
	require.Equal(t, result.RunID, start.Run.ID)
	require.Equal(t, "task-1", start.Run.BusinessTaskID)
	require.Equal(t, "tenant-a", start.Run.TenantID)
	require.Equal(t, "user-a", start.Run.UserID)
	require.Equal(t, "shein", start.Run.TargetPlatform)
	require.Equal(t, imageagent.RunModeManual, start.Run.Mode)
	require.Equal(t, 1, start.Run.MaxConcurrentSlots)
	require.Equal(t, 1, start.Run.Budget.MaxImages)
	require.Equal(t, imageagent.BudgetLimitImages, start.Run.Budget.EnabledLimits)

	plan := start.Plan
	require.EqualValues(t, 1, plan.Revision)
	require.Zero(t, plan.ParentRevision)
	require.Equal(t, []string{"source-1"}, plan.SourceAssetIDs)
	require.Empty(t, plan.StyleReferenceIDs)
	require.Len(t, plan.Slots, 1)
	require.Equal(t, "main", plan.Slots[0].ID)
	require.Equal(t, imageagent.SlotRoleMain, plan.Slots[0].Role)
	require.Equal(t, []string{"source-1"}, plan.Slots[0].SourceAssetIDs)
	require.Empty(t, plan.Slots[0].StyleReferenceIDs)
	require.NotEmpty(t, plan.IdempotencyKey)
	require.NotEmpty(t, plan.Slots[0].IdempotencyKey)
	require.NotEqual(t, plan.IdempotencyKey, plan.Slots[0].IdempotencyKey)

	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: "tenant-a", OwnerUserID: "user-a", RunID: result.RunID})
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusPlanning, projection.Run.Status)
	var persistedSourceIDs []string
	for _, asset := range projection.AssetCatalog.Assets {
		if asset.Type == imageagent.AuthorizedAssetSource {
			persistedSourceIDs = append(persistedSourceIDs, asset.ID)
		}
	}
	require.Equal(t, []string{"source-1"}, persistedSourceIDs)
}

func TestServiceLaunchTaskRunHonorsExplicitSourceAndStyleSelection(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, taskScopedCatalogResolver{catalog: launchCatalog()})
	require.NoError(t, err)

	input := launchTaskRunInput()
	input.SourceAssetID = "source-2"
	input.StyleAssetIDs = []string{"style-1"}
	result, err := service.LaunchTaskRun(verifiedContext("tenant-a", "user-a"), input)

	require.NoError(t, err)
	require.Len(t, workflows.starts, 1)
	plan := workflows.starts[0].Plan
	require.Equal(t, []string{"source-2"}, plan.SourceAssetIDs)
	require.Equal(t, []string{"style-1"}, plan.StyleReferenceIDs)
	require.Equal(t, []string{"source-2"}, plan.Slots[0].SourceAssetIDs)
	require.Equal(t, []string{"style-1"}, plan.Slots[0].StyleReferenceIDs)
	require.NotEmpty(t, result.RunID)
}

func TestServiceLaunchTaskRunRejectsUnauthorizedSourceOrStyleSelection(t *testing.T) {
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(store.NewMemoryRepository(), workflows, taskScopedCatalogResolver{catalog: launchCatalog()})
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-a")

	unknownSource := launchTaskRunInput()
	unknownSource.SourceAssetID = "source-not-authorized"
	_, err = service.LaunchTaskRun(ctx, unknownSource)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	unknownStyle := launchTaskRunInput()
	unknownStyle.SourceAssetID = "source-2"
	unknownStyle.StyleAssetIDs = []string{"source-1"}
	_, err = service.LaunchTaskRun(ctx, unknownStyle)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	require.Empty(t, workflows.starts)
}

func TestServiceLaunchTaskRunRequiresIdentityTaskAndPolicyContext(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, taskScopedCatalogResolver{catalog: launchCatalog()})
	require.NoError(t, err)

	_, err = service.LaunchTaskRun(context.Background(), launchTaskRunInput())
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)

	ctx := verifiedContext("tenant-a", "user-a")
	missingRequestID := launchTaskRunInput()
	missingRequestID.RequestID = " "
	_, err = service.LaunchTaskRun(ctx, missingRequestID)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	missingTask := launchTaskRunInput()
	missingTask.BusinessTaskID = "  "
	_, err = service.LaunchTaskRun(ctx, missingTask)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	missingPolicy := launchTaskRunInput()
	missingPolicy.ImagePolicyContext = imageagent.ImagePolicyContext{}
	_, err = service.LaunchTaskRun(ctx, missingPolicy)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	missingPlatform := launchTaskRunInput()
	missingPlatform.TargetPlatform = " "
	_, err = service.LaunchTaskRun(ctx, missingPlatform)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	missingSource := launchTaskRunInput()
	missingSource.SourceAssetID = " "
	_, err = service.LaunchTaskRun(ctx, missingSource)
	require.ErrorIs(t, err, imageagent.ErrValidation)

	require.Empty(t, workflows.starts)
}

func TestServiceLaunchTaskRunRejectsTaskWithoutAuthorizedSourceAssets(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	catalog := launchCatalog()
	catalog.Assets = catalog.Assets[2:] // style only
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: catalog})
	require.NoError(t, err)
	input := launchTaskRunInput()
	input.SourceAssetID = ""

	_, err = service.LaunchTaskRun(verifiedContext("tenant-a", "user-a"), input)

	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.Empty(t, workflows.starts)
}

// admissionGateTrackingCatalogResolver records the resolve call order so
// tests can assert admission is checked before any catalog work.
type admissionGateTrackingCatalogResolver struct {
	catalog   imageagent.AssetCatalog
	startGate *gateCallRecorder
	resolved  int
}

func (r *admissionGateTrackingCatalogResolver) Resolve(ctx context.Context, scope imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	r.resolved++
	return taskScopedCatalogResolver{catalog: r.catalog}.Resolve(ctx, scope)
}

type gateCallRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (g *gateCallRecorder) AllowTenantStart(_ context.Context, tenantID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, tenantID)
	return false
}

func TestServiceLaunchTaskRunChecksAdmissionBeforeResolvingAssets(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	gate := &gateCallRecorder{}
	catalogResolver := &admissionGateTrackingCatalogResolver{catalog: launchCatalog()}
	service, err := imageagent.NewService(repository, workflows, catalogResolver,
		imageagent.WithTenantStartGate(gate))
	require.NoError(t, err)

	_, err = service.LaunchTaskRun(verifiedContext("tenant-b", "user-a"), launchTaskRunInput())

	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Equal(t, []string{"tenant-b"}, gate.calls, "admission must be checked exactly once before any catalog work")
	require.Zero(t, catalogResolver.resolved, "catalog resolution must not run for an ineligible tenant")
	require.Empty(t, workflows.starts)
}

func TestServiceLaunchTaskRunUsesRequestIdentityForRetryDeduplication(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, taskScopedCatalogResolver{catalog: launchCatalog()})
	require.NoError(t, err)
	ctx := verifiedContext("tenant-a", "user-a")
	input := launchTaskRunInput()

	first, err := service.LaunchTaskRun(ctx, input)
	require.NoError(t, err)
	require.Len(t, workflows.starts, 1)

	// Simulates a lost HTTP response: the caller retries the identical launch.
	// The retry replays the same durable run identity; the re-dispatch goes
	// to the same Temporal workflow (USE_EXISTING), never a second paid run.
	second, err := service.LaunchTaskRun(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first.RunID, second.RunID, "retried launch must replay the same durable run identity")
	require.Len(t, workflows.starts, 2)
	require.Equal(t, first.RunID, workflows.starts[1].Run.ID, "replay must re-dispatch the same workflow identity")

	// A new user launch of the same payload must create a fresh run.
	newLaunch := launchTaskRunInput()
	newLaunch.RequestID = "launch-request-2"
	third, err := service.LaunchTaskRun(ctx, newLaunch)
	require.NoError(t, err)
	require.NotEqual(t, first.RunID, third.RunID)
	require.Len(t, workflows.starts, 3)

	// Reusing one request identity with a changed payload is a conflict, not a
	// new paid execution hidden behind the same retry key.
	changed := launchTaskRunInput()
	changed.SourceAssetID = "source-2"
	_, err = service.LaunchTaskRun(ctx, changed)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Len(t, workflows.starts, 3)
}

func TestServicePreflightTaskRunAssetsListsSelectableAssets(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, taskScopedCatalogResolver{catalog: launchCatalog()})
	require.NoError(t, err)

	preflight, err := service.PreflightTaskRunAssets(verifiedContext("tenant-a", "user-a"), imageagent.TaskRunAssetsInput{
		BusinessTaskID: "task-1", TargetPlatform: "shein",
	})

	require.NoError(t, err)
	require.Equal(t, "task-1", preflight.BusinessTaskID)
	require.Equal(t, "shein", preflight.TargetPlatform)
	require.Equal(t, []string{"source-1", "source-2"}, authorizedAssetIDs(preflight.Sources))
	require.Equal(t, []string{"style-1"}, authorizedAssetIDs(preflight.Styles))
	require.Empty(t, workflows.starts, "preflight must never dispatch a workflow")
}

func TestServicePreflightTaskRunAssetsChecksAdmissionBeforeResolvingAssets(t *testing.T) {
	workflows := &recordingWorkflowClient{}
	gate := &gateCallRecorder{}
	catalogResolver := &admissionGateTrackingCatalogResolver{catalog: launchCatalog()}
	service, err := imageagent.NewService(store.NewMemoryRepository(), workflows, catalogResolver,
		imageagent.WithTenantStartGate(gate))
	require.NoError(t, err)

	_, err = service.PreflightTaskRunAssets(verifiedContext("tenant-b", "user-a"), imageagent.TaskRunAssetsInput{
		BusinessTaskID: "task-1", TargetPlatform: "shein",
	})

	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Equal(t, []string{"tenant-b"}, gate.calls, "admission must be checked before any catalog work")
	require.Zero(t, catalogResolver.resolved, "catalog resolution must not run for an ineligible tenant")
	require.Empty(t, workflows.starts)
}

func TestServicePreflightTaskRunAssetsRequiresTaskAndPlatform(t *testing.T) {
	service, err := imageagent.NewService(store.NewMemoryRepository(), &recordingWorkflowClient{}, taskScopedCatalogResolver{catalog: launchCatalog()})
	require.NoError(t, err)

	for name, input := range map[string]imageagent.TaskRunAssetsInput{
		"missing business task": {TargetPlatform: "shein"},
		"missing platform":      {BusinessTaskID: "task-1"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := service.PreflightTaskRunAssets(verifiedContext("tenant-a", "user-a"), input)
			require.ErrorIs(t, err, imageagent.ErrValidation)
		})
	}
}

func authorizedAssetIDs(assets []imageagent.AuthorizedAsset) []string {
	ids := make([]string, 0, len(assets))
	for _, asset := range assets {
		ids = append(ids, asset.ID)
	}
	return ids
}

func TestServiceLaunchTaskRunAdmitsTenantThroughStartGate(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, taskScopedCatalogResolver{catalog: launchCatalog()},
		imageagent.WithTenantStartGate(staticTenantStartGate{allowed: map[string]bool{"tenant-a": true}}))
	require.NoError(t, err)

	_, err = service.LaunchTaskRun(verifiedContext("tenant-b", "user-a"), launchTaskRunInput())
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Empty(t, workflows.starts)

	result, err := service.LaunchTaskRun(verifiedContext("tenant-a", "user-a"), launchTaskRunInput())
	require.NoError(t, err)
	require.Len(t, workflows.starts, 1)
	require.NotEmpty(t, result.RunID)
}
