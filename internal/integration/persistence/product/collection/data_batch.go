package collectionpersistence

import (
	"context"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/product/collection"
)

// AppendDataPublication can only join the Product producer's transaction. The
// caller has already checked its live authority and persisted source receipt.
func AppendDataPublication(ctx context.Context, tx *gorm.DB, scope collection.Scope, batch collection.PublicationBatch, source collection.Source, at time.Time) (string, error) {
	verified, err := collection.NewPublicationBatch(scope, batch.OperationID, batch.Kind, batch.Name)
	if err != nil || verified != batch || source.Kind != batch.Kind || !collection.ValidID(source.OperationID) {
		return "", collection.ErrInvalid
	}
	if tx == nil {
		return "", collection.ErrUnavailable
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return "", collection.ErrUnavailable
	}
	tx = tx.WithContext(ctx)
	if err := insertBatch(tx, scope, batch.ID, batch.Name, batch.Kind, at); err != nil {
		return "", err
	}
	// Archival only hides collection rows; it does not cancel this already
	// admitted producer. Serialize with archival but retain its marker and the
	// original job/delivery batch identity. User mutations still use lockBatch.
	var locked collection.Batch
	row := tx.Raw("SELECT id,name,kind,revision,created_at,archived_at FROM product_collection_batches WHERE organization_id=? AND actor_id=? AND id=? FOR UPDATE", scope.OrganizationID, scope.ActorID, batch.ID).Scan(&locked)
	if row.Error != nil {
		return "", row.Error
	}
	if row.RowsAffected != 1 {
		return "", collection.ErrNotFound
	}
	if locked.Kind != batch.Kind || locked.Revision < 1 {
		return "", collection.ErrConflict
	}
	return appendSource(tx, scope, batch.ID, source, at)
}
