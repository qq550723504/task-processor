package supplymarketapp

import (
	"context"
	"reflect"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/review"
	"task-processor/internal/product/supplymarket"
)

type EffectiveProductReader struct {
	Snapshots catalog.VersionedSnapshotReader
	Applied   review.AppliedPublicationLookup
}

func (r EffectiveProductReader) ReadChoice(ctx context.Context, scope collection.Scope, item collection.ItemDetail) (supplymarket.ProductChoice, error) {
	if scope.Validate() != nil || item.Item.Source.Kind != "own" || ctx == nil {
		return supplymarket.ProductChoice{}, supplymarket.ErrConflict
	}
	reader, ok := r.Snapshots.(catalog.CompleteSnapshotReader)
	if !ok {
		return supplymarket.ProductChoice{}, supplymarket.ErrUnavailable
	}
	current, err := reader.GetCurrentSnapshot(ctx, catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: item.Item.Source.ProductKey})
	if err != nil {
		return supplymarket.ProductChoice{}, err
	}
	apply := ""
	if current.Version != item.Item.Source.Version {
		lineage, err := review.ResolveAppliedSnapshot(ctx, review.Scope{Org: scope.OrganizationID, Actor: scope.ActorID}, current, r.Snapshots, r.Applied)
		if err != nil || len(lineage.Applied) == 0 {
			return supplymarket.ProductChoice{}, supplymarket.ErrConflict
		}
		apply = lineage.Applied[0].ProposalID
	}
	effective, err := r.ReadEffective(ctx, scope, item, current.Version, apply)
	if err != nil {
		return supplymarket.ProductChoice{}, err
	}
	product, err := supplymarket.PublicProjection(effective.Snapshot)
	if err != nil {
		return supplymarket.ProductChoice{}, err
	}
	return supplymarket.ProductChoice{Selection: supplymarket.SelectionInput{ItemID: item.Item.ID, ExpectedRevision: item.Item.Revision, OriginalPublicationID: item.Item.Source.PublicationID, OriginalVersion: item.Item.Source.Version, EffectiveVersion: effective.Version, ApplyID: apply}, Product: product, Optimized: apply != ""}, nil
}

func (r EffectiveProductReader) ReadEffective(ctx context.Context, scope collection.Scope, item collection.ItemDetail, version uint64, applyID string) (catalog.PublishedSnapshot, error) {
	if ctx == nil || ctx.Err() != nil || scope.Validate() != nil || r.Snapshots == nil || item.Item.Source.Kind != "own" || version == 0 {
		return catalog.PublishedSnapshot{}, supplymarket.ErrConflict
	}
	identity := catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: item.Item.Source.ProductKey}
	original, err := r.Snapshots.GetSnapshot(ctx, identity, item.Item.Source.Version)
	if err != nil || original.Identity != identity || original.Version != item.Item.Source.Version || original.PublicationID != item.Item.Source.PublicationID || !reflect.DeepEqual(original.Snapshot, item.Product) {
		return catalog.PublishedSnapshot{}, supplymarket.ErrConflict
	}
	if applyID == "" {
		if version != original.Version {
			return catalog.PublishedSnapshot{}, supplymarket.ErrConflict
		}
		return original, nil
	}
	if !collection.ValidID(applyID) || version <= original.Version || r.Applied == nil {
		return catalog.PublishedSnapshot{}, supplymarket.ErrConflict
	}
	current, err := r.Snapshots.GetSnapshot(ctx, identity, version)
	if err != nil || current.Identity != identity || current.Version != version {
		return catalog.PublishedSnapshot{}, supplymarket.ErrConflict
	}
	lineage, err := review.ResolveAppliedSnapshot(ctx, review.Scope{Org: scope.OrganizationID, Actor: scope.ActorID}, current, r.Snapshots, r.Applied)
	if err != nil || len(lineage.Applied) == 0 || lineage.Applied[0].ProposalID != applyID || !reflect.DeepEqual(lineage.Original, original) {
		return catalog.PublishedSnapshot{}, supplymarket.ErrConflict
	}
	return current, ctx.Err()
}

// The original scope comes only from the locked disclosure record. This
// bounded tokenless context delegates authority to live IAM; it carries no
// impersonated organization, member identity, bearer token, or role.
func originalOwnerContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, nil, supplymarket.ErrForbidden
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return nil, nil, supplymarket.ErrForbidden
	}
	owner, cancel := context.WithDeadline(context.Background(), deadline)
	stop := context.AfterFunc(ctx, cancel)
	return owner, func() { stop(); cancel() }, nil
}
