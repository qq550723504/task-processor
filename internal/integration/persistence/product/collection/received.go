package collectionpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/product/collection"
	"time"
)

// AppendReceived is the current producer's narrow same-Product-UoW port.
// It stores one recipient-private source reference, never copies approvals or
// changes the original merchant's product. The caller rolls back on any error.
func AppendReceived(ctx context.Context, tx *gorm.DB, scope collection.Scope, operation, name string, source collection.Source, at time.Time) (collection.Receipt, error) {
	if ctx == nil || tx == nil || scope.Validate() != nil || !collection.ValidID(operation) || source.OperationID != operation || len(name) == 0 || len(name) > 200 {
		return collection.Receipt{}, collection.ErrInvalid
	}
	if source.Kind != "market" && source.Kind != "sds_template" && source.Kind != "sds_finished" {
		return collection.Receipt{}, collection.ErrInvalid
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return collection.Receipt{}, collection.ErrUnavailable
	}
	tx = tx.WithContext(ctx)
	batch := collection.StableID(operation, "batch")
	if err := insertBatch(tx, scope, batch, name, source.Kind, at); err != nil {
		return collection.Receipt{}, err
	}
	if _, err := lockBatch(tx, scope, batch, 0); err != nil {
		return collection.Receipt{}, err
	}
	item, err := appendSource(tx, scope, batch, source, at)
	if err != nil {
		return collection.Receipt{}, err
	}
	return collection.Receipt{OperationID: operation, BatchID: batch, ItemID: item, Revision: 2}, nil
}

// LockPrivateSource consumes a sealed live execution owner proof. It validates
// the exact original member and reference after locking its visible batch and
// item, using the Collection owner's existing lock order.
func LockPrivateSource(ctx context.Context, tx *gorm.DB, owner collection.AuthorizedOwner, id string, revision int64, source collection.Source) (collection.Item, error) {
	if ctx == nil || tx == nil || !collection.ValidID(id) || revision < 1 {
		return collection.Item{}, collection.ErrInvalid
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return collection.Item{}, collection.ErrUnavailable
	}
	scope, err := owner.Scope(ctx)
	if err != nil {
		return collection.Item{}, err
	}
	tx = tx.WithContext(ctx)
	var row itemRow
	found := tx.Raw("SELECT "+itemColumns+" FROM product_collection_items WHERE organization_id=? AND actor_id=? AND member_id=? AND id=? AND archived_at IS NULL", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Scan(&row)
	if found.Error != nil {
		return collection.Item{}, found.Error
	}
	if found.RowsAffected != 1 {
		return collection.Item{}, collection.ErrNotFound
	}
	if _, err := lockBatch(tx, scope, row.BatchID, 0); err != nil {
		return collection.Item{}, err
	}
	locked := tx.Raw("SELECT "+itemColumns+" FROM product_collection_items WHERE organization_id=? AND actor_id=? AND member_id=? AND id=? AND archived_at IS NULL FOR UPDATE", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Scan(&row)
	if locked.Error != nil {
		return collection.Item{}, locked.Error
	}
	if locked.RowsAffected != 1 {
		return collection.Item{}, collection.ErrNotFound
	}
	item := row.item()
	if item.Revision != revision || item.Source.ProductKey != source.ProductKey || item.Source.PublicationID != source.PublicationID || item.Source.Version != source.Version || item.Source.Kind != source.Kind {
		return collection.Item{}, collection.ErrConflict
	}
	return item, nil
}
