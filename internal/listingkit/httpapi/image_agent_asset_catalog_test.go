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

func TestListingKitImageAgentCatalogIncludesOnlyExplicitNonSourceStyles(t *testing.T) {
	task := &listingkit.Task{
		ID: "task-styles", TenantID: "tenant-a", UserID: "user-a",
		Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{
			{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/source.png"},
			{ID: "scene-1", Kind: asset.KindSceneImage, URL: "https://cdn.example.test/scene.png", Labels: []string{"Lifestyle"}},
			{ID: "generated-1", Kind: asset.KindGalleryImage, URL: "https://cdn.example.test/generated.png"},
		}}}},
	}

	got, err := imageAgentCatalogFromTask(task, []string{"scene-1"})
	require.NoError(t, err)
	require.Len(t, got.Assets, 2)
	require.Equal(t, imageagent.AuthorizedAssetSource, got.Assets[0].Type)
	require.Equal(t, imageagent.AuthorizedAssetStyle, got.Assets[1].Type)
	require.Equal(t, "scene-1", got.Assets[1].ID)

	_, err = imageAgentCatalogFromTask(task, []string{"source-1"})
	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.ErrorContains(t, err, "source asset cannot be selected as a style")

	_, err = imageAgentCatalogFromTask(task, []string{"missing-style"})
	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.ErrorContains(t, err, "unknown style asset")
}

func TestImageAgentCatalogRejectsTargetKeyedAssetBundlesWithoutTargetAuthorization(t *testing.T) {
	task := &listingkit.Task{Result: &listingkit.ListingKitResult{
		StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{{
			ID: "source-1", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/source.png",
		}}}},
		AssetBundlesByTarget: map[string]*asset.Bundle{
			"shein": {Assets: []asset.Asset{{ID: "shein-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/shein.png"}}},
		},
	}}

	_, err := imageAgentCatalogFromTask(task)
	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestImageAgentCatalogUsesOnlyExplicitOwnedTargetBundle(t *testing.T) {
	task := &listingkit.Task{Result: &listingkit.ListingKitResult{
		StandardProductSnapshot: &listingkit.StandardProductSnapshot{},
		AssetBundlesByTarget: map[string]*asset.Bundle{
			"shein":  {Assets: []asset.Asset{{ID: "shein-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/shein.png"}}},
			"amazon": {Assets: []asset.Asset{{ID: "amazon-source", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/amazon.png"}}},
		},
	}}

	got, err := imageAgentCatalogFromTaskTarget(task, " SHEIN ")
	require.NoError(t, err)
	require.Equal(t, []imageagent.AuthorizedAsset{{ID: "shein-source", Type: imageagent.AuthorizedAssetSource, URL: "https://cdn.example.test/shein.png", SourceURL: "https://cdn.example.test/shein.png", DisplayURL: "https://cdn.example.test/shein.png", Label: "Source image"}}, got.Assets)

	_, err = imageAgentCatalogFromTaskTarget(task, "temu")
	require.ErrorIs(t, err, imageagent.ErrValidation)
	_, err = imageAgentCatalogFromTaskTarget(task, "")
	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestListingKitImageAgentCatalogTruncatesDisplayLabelsAt256UnicodeCodePoints(t *testing.T) {
	longLabel := strings.Repeat("界", 257)
	task := &listingkit.Task{
		ID: "task-label", TenantID: "tenant-a", UserID: "user-a",
		Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{AssetBundle: &asset.Bundle{Assets: []asset.Asset{
			{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://cdn.example.test/source.png", Labels: []string{longLabel}},
		}}}},
	}

	got, err := imageAgentCatalogFromTask(task, nil)
	require.NoError(t, err)
	require.Len(t, []rune(got.Assets[0].Label), 256)

	task.Result.StandardProductSnapshot.AssetBundle.Assets[0].Labels[0] = strings.Repeat("界", 256)
	got, err = imageAgentCatalogFromTask(task, nil)
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("界", 256), got.Assets[0].Label)
}

type listingTaskSourceStub struct{ task *listingkit.Task }

func (s listingTaskSourceStub) GetTask(context.Context, string) (*listingkit.Task, error) {
	return s.task, nil
}
