package imageagent_test

import (
	"context"
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

// taskScopedCatalogResolver mirrors the composition-owned catalog: it narrows
// to the caller-selected primary source and promotes only the requested style
// references into the run-scoped snapshot.
type taskScopedCatalogResolver struct{ catalog imageagent.AssetCatalog }

func (r taskScopedCatalogResolver) Resolve(_ context.Context, scope imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	wantedStyles := make(map[string]struct{}, len(scope.StyleReferenceIDs))
	for _, id := range scope.StyleReferenceIDs {
		wantedStyles[id] = struct{}{}
	}
	assets := make([]imageagent.AuthorizedAsset, 0, len(r.catalog.Assets))
	for _, asset := range r.catalog.Assets {
		switch {
		case asset.Type == imageagent.AuthorizedAssetSource && scope.PrimarySourceAssetID != "" && asset.ID != scope.PrimarySourceAssetID:
			continue
		case asset.Type == imageagent.AuthorizedAssetStyle:
			if _, ok := wantedStyles[asset.ID]; !ok {
				continue
			}
		}
		assets = append(assets, asset)
	}
	return imageagent.AssetCatalog{Assets: assets}, nil
}

func launchTaskRunInput() imageagent.TaskRunLaunchInput {
	return imageagent.TaskRunLaunchInput{
		BusinessTaskID: "task-1", TargetPlatform: "shein",
		ImagePolicyContext: imageagent.ImagePolicyContext{Country: "us", Family: "default", SceneCategory: "shoes"},
	}
}

func TestServiceLaunchTaskRunStartsSingleMainSlotRunWithPrimarySource(t *testing.T) {
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
	require.Len(t, projection.AssetCatalog.Assets, 1)
	require.Equal(t, "source-1", projection.AssetCatalog.Assets[0].ID)
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

	require.Empty(t, workflows.starts)
}

func TestServiceLaunchTaskRunRejectsTaskWithoutAuthorizedSourceAssets(t *testing.T) {
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	catalog := launchCatalog()
	catalog.Assets = catalog.Assets[2:] // style only
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: catalog})
	require.NoError(t, err)

	_, err = service.LaunchTaskRun(verifiedContext("tenant-a", "user-a"), launchTaskRunInput())

	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.Empty(t, workflows.starts)
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
