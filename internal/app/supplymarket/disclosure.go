package supplymarketapp

import (
	"context"
	"gorm.io/gorm"
	"reflect"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	marketstore "task-processor/internal/integration/persistence/product/supplymarket"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/supplymarket"
)

type disclosureVerifier struct {
	tx            *gorm.DB
	authorization collection.ExecutionAuthorizer
	products      EffectiveProductReader
}

func NewDisclosureVerifier(tx *gorm.DB, authorization collection.ExecutionAuthorizer) (marketstore.DisclosureVerifier, error) {
	if tx == nil || authorization == nil {
		return nil, supplymarket.ErrUnavailable
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, supplymarket.ErrUnavailable
	}
	snapshots, err := catalogstore.NewBoundedSnapshotReader(tx, 2<<20)
	if err != nil {
		return nil, err
	}
	applied, err := reviewstore.NewAppliedPublicationReader(tx)
	if err != nil {
		return nil, err
	}
	return disclosureVerifier{tx, authorization, EffectiveProductReader{snapshots, applied}}, nil
}
func (v disclosureVerifier) Verify(ctx context.Context, source supplymarket.SourceReference, product supplymarket.PublicProduct) error {
	ownerCtx, cancel, err := originalOwnerContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	if v.authorization.AuthorizeExecution(ownerCtx, source.Scope, collection.PermissionManage) != nil {
		return supplymarket.ErrForbidden
	}
	owner, err := (collection.ExecutionOwnerAuthority{Authorization: v.authorization}).AuthorizeExecutionOwner(ownerCtx, source.Scope)
	if err != nil {
		return supplymarket.ErrForbidden
	}
	item, err := collectionstore.LockPrivateSource(ownerCtx, v.tx, owner, source.ItemID, source.ItemRevision, collection.Source{ProductKey: source.ProductKey, PublicationID: source.OriginalPublicationID, Version: source.OriginalVersion, Kind: "own"})
	if err != nil {
		return err
	}
	original, err := v.products.Snapshots.GetSnapshot(ownerCtx, catalog.SnapshotIdentity{TenantID: source.Scope.OrganizationID, ProductKey: source.ProductKey}, source.OriginalVersion)
	if err != nil {
		return supplymarket.ErrConflict
	}
	effective, err := v.products.ReadEffective(ownerCtx, source.Scope, collection.ItemDetail{Item: item, Product: original.Snapshot}, source.Version, source.ApplyID)
	if err != nil || effective.PublicationID != source.PublicationID {
		return supplymarket.ErrConflict
	}
	exact, err := supplymarket.PublicProjection(effective.Snapshot)
	if err != nil || !reflect.DeepEqual(exact, product) {
		return supplymarket.ErrConflict
	}
	return ownerCtx.Err()
}
