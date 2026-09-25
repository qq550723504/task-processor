package httpapi

import (
	"context"
	"strings"

	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/sourcing"
)

// organizationImageCatalog derives every image input from the published
// acquisition receipt for the current actor and organization. The operation ID
// occupies ImageAgent's existing context transport field; it is not a Task.
type organizationImageCatalog struct {
	receipts interface {
		ReadPublished(context.Context, string) (productsourcing.PublishedAcquisition, error)
	}
}

func (c organizationImageCatalog) Resolve(ctx context.Context, scope imageagent.AssetCatalogScope) (imageagent.AssetCatalog, error) {
	if err := c.validateIdentity(ctx, scope); err != nil {
		return imageagent.AssetCatalog{}, err
	}
	if scope.PrimarySourceAssetID == "" || len(scope.StyleReferenceIDs) != 0 {
		return imageagent.AssetCatalog{}, imageagent.ErrValidation
	}
	published, err := c.readPublished(ctx, scope)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	assets, err := selectAuthorizedAssets(buildAuthorizedAssetsFromCatalogImages(published.Snapshot.Snapshot.Images), scope.PrimarySourceAssetID, nil, true)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	result, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{
		Assets: assets,
		ProductContext: imageagent.ProductContextRef{
			ProductID:             published.Snapshot.Identity.ProductKey,
			Title:                 published.Snapshot.Snapshot.Title,
			ProductType:           strings.Join(published.Snapshot.Snapshot.CategoryPath, " / "),
			SourceSnapshotVersion: published.Snapshot.Version,
			Attributes:            catalogAttributes(published.Snapshot.Snapshot.Attributes),
		},
	})
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	result.Manifest.Hash = imageagent.CatalogSnapshotHash(result.Assets, result.ProductContext)
	return result, nil
}

// Candidates projects selectable IDs from the same authoritative receipt and
// original image order that Resolve later rechecks. URLs are display-only.
func (c organizationImageCatalog) Candidates(ctx context.Context, scope imageagent.AssetCatalogScope) ([]imageagent.AuthorizedAsset, error) {
	published, err := c.readPublished(ctx, scope)
	if err != nil {
		return nil, err
	}
	var sources []imageagent.AuthorizedAsset
	for _, asset := range buildAuthorizedAssetsFromCatalogImages(published.Snapshot.Snapshot.Images) {
		if asset.Type == imageagent.AuthorizedAssetSource {
			sources = append(sources, asset)
		}
	}
	return sources, nil
}

func (c organizationImageCatalog) readPublished(ctx context.Context, scope imageagent.AssetCatalogScope) (productsourcing.PublishedAcquisition, error) {
	if err := c.validateIdentity(ctx, scope); err != nil {
		return productsourcing.PublishedAcquisition{}, err
	}
	published, err := c.receipts.ReadPublished(ctx, scope.BusinessTaskID)
	if err != nil {
		return productsourcing.PublishedAcquisition{}, err
	}
	receipt := published.Result.Publication
	if published.Result.Operation.ID != scope.BusinessTaskID || published.Result.Operation.State != sourcing.AcquisitionPublished ||
		published.Result.Operation.Scope.OrganizationID != scope.TenantID || published.Result.Operation.Scope.ActorID != scope.OwnerUserID ||
		receipt == nil || receipt.Receipt.OrganizationID != scope.TenantID || receipt.Receipt.ActorID != scope.OwnerUserID ||
		published.Snapshot.Identity.TenantID != scope.TenantID || published.Snapshot.Identity.ProductKey != receipt.Receipt.ProductKey ||
		published.Snapshot.Version != receipt.Receipt.CatalogVersion || published.Snapshot.PublicationID != receipt.Receipt.CatalogPublicationID {
		return productsourcing.PublishedAcquisition{}, imageagent.ErrIdentityRequired
	}
	return published, nil
}

func (c organizationImageCatalog) validateIdentity(ctx context.Context, scope imageagent.AssetCatalogScope) error {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.EffectiveOrganizationID == "" || identity.TenantID != identity.EffectiveOrganizationID ||
		identity.TenantID != scope.TenantID || identity.UserID != scope.OwnerUserID ||
		strings.TrimSpace(scope.BusinessTaskID) == "" || c.receipts == nil {
		return imageagent.ErrIdentityRequired
	}
	return nil
}
