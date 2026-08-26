package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
)

func TestListingKitImageAgentCatalogUsesOwnedCanonicalSourceAssetsAndNoSyntheticStyles(t *testing.T) {
	tasks := listingTaskSourceStub{task: &listingkit.Task{
		ID: "task-1", TenantID: "tenant-a", UserID: "user-a",
		Request: &listingkit.GenerateRequest{ImageURLs: []string{"https://source.example/request-only.png"}},
		Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{
			{ID: "source-canonical", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/source.png", Labels: []string{"Canonical source"}, Width: 1200, Height: 900},
			{ID: "generated-1", Kind: asset.KindSceneImage, URL: "https://cdn.example.test/generated.png"},
		}}}},
	}}
	catalog := newListingKitAuthorizedAssetCatalog(tasks)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})

	got, err := catalog.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", BusinessTaskID: "task-1", RunID: "run-1"})
	require.NoError(t, err)
	require.Equal(t, imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-canonical", Type: imageagent.AuthorizedAssetSource, DisplayURL: "https://cdn.example.test/source.png", Label: "Canonical source", Width: 1200, Height: 900}}}, got)
	_, err = catalog.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-b", BusinessTaskID: "task-1", RunID: "run-1"})
	require.Error(t, err)

	requestOnly := newListingKitAuthorizedAssetCatalog(listingTaskSourceStub{task: &listingkit.Task{
		ID: "task-request-only", TenantID: "tenant-a",
		Request: &listingkit.GenerateRequest{ImageURLs: []string{"https://source.example/request-only.png"}},
	}})
	_, err = requestOnly.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", BusinessTaskID: "task-request-only", RunID: "run-2"})
	require.ErrorContains(t, err, "no authorized source assets")
}

type listingTaskSourceStub struct{ task *listingkit.Task }

func (s listingTaskSourceStub) GetTask(context.Context, string) (*listingkit.Task, error) {
	return s.task, nil
}
