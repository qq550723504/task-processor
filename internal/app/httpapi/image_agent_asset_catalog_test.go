package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/catalog"
)

func TestAppImageAgentCatalogReadsOnlyProductSnapshotCatalogImages(t *testing.T) {
	task := catalogTaskFixture()
	got, err := imageAgentCatalogFromTask(task, []string{"catalog-image-2"})
	require.NoError(t, err)
	require.Equal(t, "product-1", got.ProductContext.ProductID)
	require.Equal(t, "Travel Bottle", got.ProductContext.Title)
	require.Equal(t, "Outdoors / Bottles", got.ProductContext.ProductType)
	require.Equal(t, "Steel", got.ProductContext.Attributes["Material"])
	require.Equal(t, []string{"catalog-image-1", "catalog-image-3", "catalog-image-2"}, []string{got.Assets[0].ID, got.Assets[1].ID, got.Assets[2].ID})
	require.Equal(t, imageagent.AuthorizedAssetSource, got.Assets[0].Type)
	require.Equal(t, imageagent.AuthorizedAssetStyle, got.Assets[2].Type)
}

func TestAppImageAgentCatalogUsesExplicitSourceSelection(t *testing.T) {
	task := catalogTaskFixture()
	got, err := imageAgentCatalogFromTaskTargetSelection(task, "", "catalog-image-3", []string{"catalog-image-2"})
	require.NoError(t, err)
	require.Equal(t, []string{"catalog-image-3", "catalog-image-2"}, []string{got.Assets[0].ID, got.Assets[1].ID})

	_, err = imageAgentCatalogFromTaskTargetSelection(task, "", "missing", nil)
	require.ErrorIs(t, err, imageagent.ErrValidation)
}

func TestAppImageAgentCatalogRequiresOwnedTaskAndSnapshot(t *testing.T) {
	task := catalogTaskFixture()
	resolver := newImageAgentAuthorizedAssetCatalog(listingTaskSourceStub{task: task})
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-a", UserID: "user-a"})
	_, err := resolver.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", OwnerUserID: "user-a", BusinessTaskID: "task-1"})
	require.NoError(t, err)

	_, err = resolver.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-b", OwnerUserID: "user-a", BusinessTaskID: "task-1"})
	require.Error(t, err)

	task.Result.StandardProductSnapshot.CatalogProduct = nil
	_, err = resolver.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: "tenant-a", OwnerUserID: "user-a", BusinessTaskID: "task-1"})
	require.ErrorContains(t, err, "product snapshot")
}

func catalogTaskFixture() *listingkit.Task {
	return &listingkit.Task{
		ID: "task-1", TenantID: "tenant-a", UserID: "user-a",
		Request: &listingkit.GenerateRequest{ProductKey: "product-1"},
		Result: &listingkit.ListingKitResult{StandardProductSnapshot: &listingkit.StandardProductSnapshot{
			CatalogProduct: &catalog.ProductSnapshot{
				Title: "Travel Bottle", CategoryPath: []string{"Outdoors", "Bottles"},
				Attributes: []catalog.Attribute{{Name: "Material", Value: "Steel"}},
				Images: []catalog.Image{
					{URL: "https://cdn.example.test/source.png", Role: "source"},
					{URL: "https://cdn.example.test/style.png", Role: "style"},
					{URL: "https://cdn.example.test/source-2.png", Role: "source"},
					{URL: "javascript:alert(1)", Role: "source"},
				},
			},
		}},
	}
}

type listingTaskSourceStub struct{ task *listingkit.Task }

func (s listingTaskSourceStub) GetTask(context.Context, string) (*listingkit.Task, error) {
	return s.task, nil
}
