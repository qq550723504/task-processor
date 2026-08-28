package httpapi

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/asset"
	"task-processor/internal/authidentity"
	"task-processor/internal/catalog"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
)

func TestListingKitImageAgentCatalogPreservesBusinessSourceIDOutsideObjectKeyGrammar(t *testing.T) {
	sourceID := "source:" + strings.Repeat("x", 121)
	tasks := listingTaskSourceStub{task: &listingkit.Task{
		ID: "task-1", TenantID: "tenant-a", UserID: "user-a",
		Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{{
			ID: sourceID, Kind: asset.KindSourceImage, URL: "https://cdn.example.test/source.png",
		}}}}},
	}}
	resolver := NewImageAgentAuthorizedAssetCatalog(tasks)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})

	got, err := resolver.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", OwnerUserID: "user-a", BusinessTaskID: "task-1", RunID: "run-1"})

	require.NoError(t, err)
	require.Equal(t, sourceID, got.Assets[0].ID)
}

func TestListingKitImageAgentCatalogUsesOwnedCanonicalSourceAssetsAndNoSyntheticStyles(t *testing.T) {
	tasks := listingTaskSourceStub{task: &listingkit.Task{
		ID: "task-1", TenantID: "tenant-a", UserID: "user-a",
		Request: &listingkit.GenerateRequest{ImageURLs: []string{"https://source.example/request-only.png"}},
		Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{CatalogProduct: &catalog.Product{Title: " Travel Bottle ", CategoryPath: []string{"Outdoors", " Bottles "}, Attributes: []catalog.Attribute{{Name: " Material ", Value: " Steel "}, {Name: "", Value: "ignored"}}}, AssetBundle: &asset.Bundle{Assets: []asset.Asset{
			{ID: "source-canonical", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/source.png", Labels: []string{"Canonical source"}, Width: 1200, Height: 900, Metadata: map[string]string{"origin": "canonical"}},
			{ID: "source-unsafe", Kind: asset.KindSourceImage, URL: "javascript:alert(1)"},
			{ID: "generated-1", Kind: asset.KindSceneImage, URL: "https://cdn.example.test/generated.png"},
		}}}},
	}}
	catalog := NewImageAgentAuthorizedAssetCatalog(tasks)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})

	got, err := catalog.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", OwnerUserID: "user-a", BusinessTaskID: "task-1", RunID: "run-1"})
	require.NoError(t, err)
	want, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{
		Assets:         []imageagent.AuthorizedAsset{{ID: "source-canonical", Type: imageagent.AuthorizedAssetSource, URL: "https://cdn.example.test/source.png", SourceURL: "https://cdn.example.test/source.png", DisplayURL: "https://cdn.example.test/source.png", Label: "Canonical source", Width: 1200, Height: 900}},
		ProductContext: imageagent.ProductContextRef{ProductID: "task-1", Title: "Travel Bottle", ProductType: "Bottles", Attributes: map[string]string{"Material": "Steel"}},
	})
	require.NoError(t, err)
	require.Equal(t, want, got)
	tasks.task.Result.StandardProductSnapshot.AssetBundle.Assets[0].Metadata["origin"] = "mutated-after-resolve"
	tasks.task.Result.StandardProductSnapshot.CatalogProduct.Attributes[0].Value = "mutated-after-resolve"
	require.Nil(t, got.Assets[0].Metadata, "unclassified task metadata must not enter provider authorization input")
	require.Equal(t, "Steel", got.ProductContext.Attributes["Material"], "provider context must be an immutable run snapshot")
	_, err = catalog.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-b", OwnerUserID: "user-a", BusinessTaskID: "task-1", RunID: "run-1"})
	require.Error(t, err)
	_, err = catalog.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", OwnerUserID: "user-b", BusinessTaskID: "task-1", RunID: "run-1"})
	require.Error(t, err)

	requestOnly := NewImageAgentAuthorizedAssetCatalog(listingTaskSourceStub{task: &listingkit.Task{
		ID: "task-request-only", TenantID: "tenant-a", UserID: "user-a",
		Request: &listingkit.GenerateRequest{ImageURLs: []string{"https://source.example/request-only.png"}},
		Result:  &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{}},
	}})
	_, err = requestOnly.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", OwnerUserID: "user-a", BusinessTaskID: "task-request-only", RunID: "run-2"})
	require.ErrorContains(t, err, "no authorized source assets")

	legacyOnly := NewImageAgentAuthorizedAssetCatalog(listingTaskSourceStub{task: &listingkit.Task{
		ID: "task-legacy-only", TenantID: "tenant-a", UserID: "user-a",
		Result: &listingkit.ListingKitResult{
			AssetBundle: &asset.Bundle{Assets: []asset.Asset{{
				ID: "legacy-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/legacy.png", Width: 1200, Height: 900,
			}}},
		},
	}})
	_, err = legacyOnly.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", OwnerUserID: "user-a", BusinessTaskID: "task-legacy-only", RunID: "run-3"})
	require.ErrorContains(t, err, "standard product snapshot")
}

type listingTaskSourceStub struct{ task *listingkit.Task }

func (s listingTaskSourceStub) GetTask(context.Context, string) (*listingkit.Task, error) {
	return s.task, nil
}
