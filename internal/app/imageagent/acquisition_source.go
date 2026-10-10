package imageagentapp

import (
	"context"
	"reflect"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/review"
	"task-processor/internal/product/sourcing"
)

type EffectiveImageProductReader struct {
	Snapshots catalog.VersionedSnapshotReader
	Applied   review.AppliedPublicationLookup
}

type AcquisitionImageSetSources struct {
	Receipts      sourcing.PublishedAcquisitionReader
	Authorization imageagent.ExecutionAuthorizer
	Products      EffectiveImageProductReader
}

func (r AcquisitionImageSetSources) ReadImageSetSource(ctx context.Context, identity imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if ctx == nil || r.Receipts == nil || r.Authorization == nil || !ok || identity.ScopeProtocol != imageagent.OrganizationScopeProtocol || input.ContextID != identity.BusinessTaskID || input.ContextID == "" || verified.TenantID != identity.TenantID || verified.EffectiveOrganizationID != identity.TenantID || verified.UserID != identity.UserID || verified.EffectiveMemberID != identity.MemberID || identity.MemberID == "" {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	if err := r.Authorization.AuthorizeExecution(ctx, identity); err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	published, err := r.Receipts.ReadPublished(ctx, input.ContextID)
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	operation, publication, original := published.Result.Operation, published.Result.Publication, published.Snapshot
	if operation.ID != input.ContextID || operation.State != sourcing.AcquisitionPublished || operation.Scope.OrganizationID != identity.TenantID || operation.Scope.ActorID != identity.UserID || publication == nil || publication.Receipt.OrganizationID != identity.TenantID || publication.Receipt.ActorID != identity.UserID || original.Identity.TenantID != identity.TenantID || original.Identity.ProductKey != publication.Receipt.ProductKey || original.PublicationID != publication.Receipt.CatalogPublicationID || original.Version != publication.Receipt.CatalogVersion || original.Version == 0 {
		return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
	}
	effective, err := r.Products.ReadEffectiveImageProduct(ctx, identity, original, input.EffectiveCatalogVersion, input.ApplyReceiptID)
	if err != nil {
		return imageagent.ImageSetPreparation{}, err
	}
	images := append([]catalog.Image(nil), original.Snapshot.Images...)
	for _, variant := range original.Snapshot.Variants {
		images = append(images, variant.Images...)
	}
	assets := make([]imageagent.AuthorizedAsset, 0, len(images))
	for _, image := range catalog.IdentifyImages(images) {
		url, safe := imageagent.ValidateSafeImageURL(image.URL)
		if safe != nil || strings.EqualFold(strings.TrimSpace(image.Role), "style") {
			continue
		}
		assets = append(assets, imageagent.AuthorizedAsset{ID: image.ID, Type: imageagent.AuthorizedAssetSource, URL: url, SourceURL: url, DisplayURL: url, Label: "商品原始素材", Width: image.Width, Height: image.Height})
	}
	prepared, err := PrepareImageProduct(input.ContextID, original, effective, input.ApplyReceiptID, assets)
	prepared.Source.ContextKind = imageagent.ImageSourceAcquisition
	return prepared, err
}

func (r EffectiveImageProductReader) ReadEffectiveImageProduct(ctx context.Context, identity imageagent.ExecutionIdentity, original catalog.PublishedSnapshot, version uint64, applyID string) (catalog.PublishedSnapshot, error) {
	if ctx == nil || original.Identity.TenantID != identity.TenantID {
		return catalog.PublishedSnapshot{}, imageagent.ErrIdentityRequired
	}
	if version == 0 {
		version = original.Version
	}
	if applyID == "" {
		if version != original.Version {
			return catalog.PublishedSnapshot{}, imageagent.ErrRevisionConflict
		}
		return original, nil
	}
	if version <= original.Version || r.Snapshots == nil || r.Applied == nil {
		return catalog.PublishedSnapshot{}, imageagent.ErrRevisionConflict
	}
	effective, err := r.Snapshots.GetSnapshot(ctx, original.Identity, version)
	if err != nil || effective.Identity != original.Identity || effective.Version != version {
		return catalog.PublishedSnapshot{}, imageagent.ErrRevisionConflict
	}
	lineage, err := review.ResolveAppliedSnapshot(ctx, review.Scope{Org: identity.TenantID, Actor: identity.UserID}, effective, r.Snapshots, r.Applied)
	if err != nil || len(lineage.Applied) == 0 || lineage.Applied[0].ProposalID != applyID || !reflect.DeepEqual(lineage.Original, original) {
		return catalog.PublishedSnapshot{}, imageagent.ErrRevisionConflict
	}
	return effective, nil
}
