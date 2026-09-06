package catalogpersistence

import (
	"context"
	"errors"
	"task-processor/internal/product/catalog"

	"gorm.io/gorm"
)

// NewTransactionWriter participates in a caller-owned transaction. The caller
// must roll back on ANY Publisher or later receipt error; this writer never commits.
func NewTransactionWriter(tx *gorm.DB) (catalog.SnapshotWriter, error) {
	if tx == nil {
		return nil, catalog.ErrRepositoryUnavailable
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, errors.New("Catalog requires a live caller transaction")
	}
	return &repository{db: tx, callerTransaction: true}, nil
}
func (r *repository) transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	if r.callerTransaction {
		return fn(r.db.WithContext(ctx))
	}
	return r.db.WithContext(ctx).Transaction(fn)
}
