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
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.EffectiveOrganizationID == "" || identity.TenantID != identity.EffectiveOrganizationID ||
		identity.TenantID != scope.TenantID || identity.UserID != scope.OwnerUserID ||
		strings.TrimSpace(scope.BusinessTaskID) == "" || c.receipts == nil {
		return imageagent.AssetCatalog{}, imageagent.ErrIdentityRequired
	}
	if scope.PrimarySourceAssetID == "" || len(scope.StyleReferenceIDs) != 0 {
		return imageagent.AssetCatalog{}, imageagent.ErrValidation
	}
	published, err := c.receipts.ReadPublished(ctx, scope.BusinessTaskID)
	if err != nil {
		return imageagent.AssetCatalog{}, err
	}
	receipt := published.Result.Publication
	if published.Result.Operation.ID != scope.BusinessTaskID || published.Result.Operation.State != sourcing.AcquisitionPublished ||
		published.Result.Operation.Scope.OrganizationID != scope.TenantID || published.Result.Operation.Scope.ActorID != scope.OwnerUserID ||
		receipt == nil || receipt.Receipt.OrganizationID != scope.TenantID || receipt.Receipt.ActorID != scope.OwnerUserID ||
		published.Snapshot.Identity.TenantID != scope.TenantID || published.Snapshot.Identity.ProductKey != receipt.Receipt.ProductKey ||
		published.Snapshot.Version != receipt.Receipt.CatalogVersion || published.Snapshot.PublicationID != receipt.Receipt.CatalogPublicationID {
		return imageagent.AssetCatalog{}, imageagent.ErrIdentityRequired
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
