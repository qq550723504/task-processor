package httpapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

type acquisitionImageReceiptSpy struct {
	published productsourcing.PublishedAcquisition
	calls     int
}

func (s *acquisitionImageReceiptSpy) ReadPublished(context.Context, string) (productsourcing.PublishedAcquisition, error) {
	s.calls++
	return s.published, nil
}

func TestOrganizationImageCatalogUsesActorScopedReceiptAndExactSnapshot(t *testing.T) {
	const operationID = "66f51fae-0b0e-4206-b4c2-2a589f68ca03"
	const organizationID = "org-a"
	const actorID = "user-a"
	spy := &acquisitionImageReceiptSpy{published: productsourcing.PublishedAcquisition{
		Result: sourcing.AcquisitionResult{
			Operation:   sourcing.AcquisitionOperation{ID: operationID, Scope: sourcing.PublicationScope{OrganizationID: organizationID, ActorID: actorID}, State: sourcing.AcquisitionPublished},
			Publication: &sourcing.PersistedPublication{Receipt: sourcing.PublicationReceipt{OrganizationID: organizationID, ActorID: actorID, ProductKey: "crawler:1688:123", CatalogVersion: 4, CatalogPublicationID: "source-run:acquisition:" + operationID}},
		},
		Snapshot: catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: organizationID, ProductKey: "crawler:1688:123"}, Version: 4, PublicationID: "source-run:acquisition:" + operationID, Snapshot: catalog.ProductSnapshot{Title: "Sample", Images: []catalog.Image{
			{URL: "https://images.example.com/a.png", Width: 800, Height: 800},
			{URL: "http://127.0.0.1/private.png"},
			{URL: "https://images.example.com/b.png", Width: 800, Height: 800},
		}}},
	}}
	resolver := organizationImageCatalog{receipts: spy}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: organizationID, EffectiveOrganizationID: organizationID, UserID: actorID, EffectiveMemberID: "member-a"})
	result, err := resolver.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: organizationID, OwnerUserID: actorID, BusinessTaskID: operationID, PrimarySourceAssetID: "catalog-image-3"})
	require.NoError(t, err)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, "crawler:1688:123", result.ProductContext.ProductID)
	require.Equal(t, uint64(4), result.ProductContext.SourceSnapshotVersion)
	require.Len(t, result.Assets, 1)
	require.Equal(t, "catalog-image-3", result.Assets[0].ID)
	require.NotEmpty(t, result.Manifest.Hash)
	candidates, err := resolver.Candidates(ctx, imageagent.AssetCatalogScope{TenantID: organizationID, OwnerUserID: actorID, BusinessTaskID: operationID})
	require.NoError(t, err)
	require.Len(t, candidates, 2)
	require.Equal(t, []string{"catalog-image-1", "catalog-image-3"}, []string{candidates[0].ID, candidates[1].ID})
	_, err = resolver.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: organizationID, OwnerUserID: actorID, BusinessTaskID: operationID})
	require.ErrorIs(t, err, imageagent.ErrValidation)

	_, err = resolver.Resolve(ctx, imageagent.AssetCatalogScope{TenantID: organizationID, OwnerUserID: actorID, BusinessTaskID: operationID, PrimarySourceAssetID: "catalog-image-2"})
	require.ErrorIs(t, err, imageagent.ErrValidation)
	other := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: organizationID, EffectiveOrganizationID: organizationID, UserID: "user-b"})
	_, err = resolver.Resolve(other, imageagent.AssetCatalogScope{TenantID: organizationID, OwnerUserID: actorID, BusinessTaskID: operationID})
	require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
	require.Equal(t, 3, spy.calls)
}
