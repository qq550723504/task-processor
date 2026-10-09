package supplychainapp

import (
	"context"
	"reflect"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/review"
)

// Only the exact title Review.Apply receipt can select a non-original Catalog
// version. A Catalog key/version alone is neither access nor approval.
type EffectiveProductReader struct {
	Reviews   review.Store
	Snapshots catalog.VersionedSnapshotReader
}

func (r EffectiveProductReader) ReadEffectiveTargetProduct(ctx context.Context, selected preparation.AuthorizedSource, version uint64, applyID string) (catalog.PublishedSnapshot, error) {
	scope, source, original, err := selected.Read(ctx)
	if err != nil {
		return catalog.PublishedSnapshot{}, err
	}
	if applyID == "" {
		if version != original.Version {
			return catalog.PublishedSnapshot{}, record.ErrNotReady
		}
		return original, nil
	}
	if r.Reviews == nil || r.Snapshots == nil || !collection.ValidID(applyID) || version <= original.Version {
		return catalog.PublishedSnapshot{}, record.ErrNotReady
	}
	current, err := r.Snapshots.GetSnapshot(ctx, original.Identity, version)
	if err != nil || current.Identity != original.Identity || current.Version != version {
		return catalog.PublishedSnapshot{}, record.ErrNotReady
	}
	lookup, ok := r.Reviews.(review.AppliedPublicationLookup)
	if !ok {
		return catalog.PublishedSnapshot{}, record.ErrNotReady
	}
	lineage, err := review.ResolveAppliedSnapshot(ctx, review.Scope{Org: scope.OrganizationID, Actor: scope.ActorID}, current, r.Snapshots, lookup)
	if err != nil || len(lineage.Applied) == 0 || lineage.Applied[0].ProposalID != applyID || lineage.Original.Identity != original.Identity || lineage.Original.Version != original.Version || lineage.Original.PublicationID != original.PublicationID || !reflect.DeepEqual(lineage.Original.Snapshot, original.Snapshot) || current.Identity.ProductKey != source.Source.ProductKey {
		return catalog.PublishedSnapshot{}, record.ErrNotReady
	}
	return current, nil
}

type SourceImageReader struct {
	Sources       record.TargetSourceSelector
	Products      record.EffectiveTargetProductReader
	Authorization preparation.Authorizer
}

func (r SourceImageReader) ReadSourceSelection(ctx context.Context, input asset.SourceSelectionRequest) (asset.SourceSelection, error) {
	if ctx == nil || r.Sources == nil || r.Products == nil || r.Authorization == nil || input.TargetPlatform != "shein" && input.TargetPlatform != "product" {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	scope, err := r.Authorization.Authorize(ctx, preparation.PermissionManage)
	if err != nil {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	selected, err := r.Sources.Select(ctx, input.ItemID)
	if err != nil {
		return asset.SourceSelection{}, asset.ErrSourceApprovalForbidden
	}
	owner, source, original, err := selected.Read(ctx)
	if err != nil || owner != scope || source.ID != input.ItemID || original.PublicationID != input.OriginalPublicationID || original.Version != input.OriginalSnapshotVersion {
		return asset.SourceSelection{}, asset.ErrApprovalConflict
	}
	if _, err = r.Products.ReadEffectiveTargetProduct(ctx, selected, input.EffectiveCatalogVersion, input.ApplyReceiptID); err != nil {
		return asset.SourceSelection{}, asset.ErrApprovalConflict
	}
	result := asset.SourceSelection{TenantID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ItemID: source.ID, ProductKey: source.Source.ProductKey, OriginalPublicationID: original.PublicationID, OriginalSnapshotVersion: original.Version, EffectiveCatalogVersion: input.EffectiveCatalogVersion, TargetPlatform: input.TargetPlatform, Images: SourceImages(original)}
	result.ApplyReceiptID = input.ApplyReceiptID
	return result, nil
}
func SourceImages(snapshot catalog.PublishedSnapshot) []asset.SourceImage {
	result := make([]asset.SourceImage, 0, len(snapshot.Snapshot.Images))
	seen := map[string]bool{}
	images := append([]catalog.Image(nil), snapshot.Snapshot.Images...)
	for _, variant := range snapshot.Snapshot.Variants {
		images = append(images, variant.Images...)
	}
	for _, image := range images {
		if image.URL == "" || seen[image.URL] {
			continue
		}
		seen[image.URL] = true
		id := collection.StableID(snapshot.Identity.TenantID, snapshot.PublicationID, "source-image", image.URL)
		result = append(result, asset.SourceImage{ID: id, URL: image.URL, Width: image.Width, Height: image.Height, ReferenceHash: asset.ReferenceHash(id, image.URL)})
	}
	return result
}

var _ record.EffectiveTargetProductReader = EffectiveProductReader{}
var _ asset.SourceSelectionReader = SourceImageReader{}
