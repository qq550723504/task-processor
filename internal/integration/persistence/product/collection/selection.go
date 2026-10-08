package collectionpersistence

import (
	"context"
	"gorm.io/gorm"
	"task-processor/internal/product/collection"
)

type SelectionReader struct{ tx *gorm.DB }

func NewSelectionReader(tx *gorm.DB) (*SelectionReader, error) {
	if tx == nil || tx.Dialector.Name() != "postgres" {
		return nil, collection.ErrUnavailable
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, collection.ErrUnavailable
	}
	return &SelectionReader{tx: tx}, nil
}

func (r *SelectionReader) VisitSelection(ctx context.Context, proof collection.AuthorizedBatchSelection, yield func(collection.Item) error) (int64, error) {
	scope, selection, err := proof.Read(ctx)
	if err != nil {
		return 0, err
	}
	if r == nil || r.tx == nil || yield == nil {
		return 0, collection.ErrUnavailable
	}
	tx := r.tx.WithContext(ctx)
	if _, err := lockBatch(tx, scope, selection.BatchID, selection.ExpectedRevision); err != nil {
		return 0, err
	}
	var count int64
	after := ""
	for {
		if _, _, err := proof.Read(ctx); err != nil {
			return 0, err
		}
		query := tx.Table("product_collection_items").Where("organization_id=? AND actor_id=? AND batch_id=? AND archived_at IS NULL", scope.OrganizationID, scope.ActorID, selection.BatchID)
		if len(selection.ItemIDs) > 0 {
			query = query.Where("id IN ?", selection.ItemIDs)
		}
		if after != "" {
			query = query.Where("id>?::uuid", after)
		}
		var rows []itemRow
		if err := query.Select(itemColumns).Order("id").Limit(100).Find(&rows).Error; err != nil {
			return 0, err
		}
		for _, row := range rows {
			if err := yield(row.item()); err != nil {
				return 0, err
			}
			count++
		}
		if len(rows) < 100 {
			break
		}
		after = rows[len(rows)-1].ID
	}
	if count == 0 || len(selection.ItemIDs) > 0 && count != int64(len(selection.ItemIDs)) {
		return 0, collection.ErrConflict
	}
	if _, _, err := proof.Read(ctx); err != nil {
		return 0, err
	}
	return count, nil
}

var _ collection.SelectionReader = (*SelectionReader)(nil)
