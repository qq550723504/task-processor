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
	locked, err := lockBatch(tx, scope, batch.ID, 0)
	if err != nil {
		return "", err
	}
	if locked.Kind != batch.Kind {
		return "", collection.ErrConflict
	}
	return appendSource(tx, scope, batch.ID, source, at)
}
