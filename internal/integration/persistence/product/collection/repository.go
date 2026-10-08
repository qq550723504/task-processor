package collectionpersistence

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
)

type OwnPublisher interface {
	PublishOwn(context.Context, collection.Scope, string, sourcing.SourceEnvelope) (collection.Source, error)
}
type OwnPublisherFactory func(*gorm.DB) (OwnPublisher, error)
type Repository struct {
	db          *gorm.DB
	own         OwnPublisherFactory
	now         func() time.Time
	afterCommit func() error
}

func NewRepository(ctx context.Context, db *gorm.DB, own OwnPublisherFactory) (*Repository, error) {
	if own == nil || VerifySchema(ctx, db) != nil {
		return nil, collection.ErrUnavailable
	}
	return &Repository{db: db, own: own, now: time.Now}, nil
}
func (r *Repository) Execute(ctx context.Context, command collection.Command) (collection.Receipt, error) {
	if command.Scope.Validate() != nil || !collection.ValidID(command.Key) || !collection.ValidID(command.OperationID) || len(command.InputHash) != 64 {
		return collection.Receipt{}, collection.ErrInvalid
	}
	var receipt collection.Receipt
	err := r.write(ctx, func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", collection.Digest([]string{command.Scope.OrganizationID, command.Scope.ActorID, command.Key})).Error; err != nil {
			return err
		}
		var op struct {
			InputHash   string
			ReceiptJSON []byte
		}
		found := tx.Raw("SELECT input_hash,receipt_json FROM product_collection_operations WHERE organization_id=? AND actor_id=? AND command_key=?", command.Scope.OrganizationID, command.Scope.ActorID, command.Key).Scan(&op)
		if found.Error != nil {
			return found.Error
		}
		if found.RowsAffected == 1 {
			if op.InputHash != command.InputHash {
				return collection.ErrConflict
			}
			if json.Unmarshal(op.ReceiptJSON, &receipt) != nil || receipt.OperationID != command.OperationID {
				return collection.ErrUnavailable
			}
			receipt.Replayed = true
			return nil
		}
		receipt = collection.Receipt{OperationID: command.OperationID}
		input, scope, now := command.Mutation, command.Scope, r.now().UTC().Truncate(time.Microsecond)
		switch input.Action {
		case "create_batch":
			receipt.BatchID = collection.StableID(command.OperationID, "batch")
			receipt.Revision = 1
			if err := insertBatch(tx, scope, receipt.BatchID, input.Name, "manual", now); err != nil {
				return err
			}
		case "rename_batch", "archive_batch":
			if _, err := lockBatch(tx, scope, input.BatchID, input.ExpectedRevision); err != nil {
				return err
			}
			query, value := "UPDATE product_collection_batches SET name=?,revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", any(input.Name)
			if input.Action == "archive_batch" {
				query, value = "UPDATE product_collection_batches SET archived_at=?,revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", now
			}
			if err := tx.Exec(query, value, scope.OrganizationID, scope.ActorID, input.BatchID).Error; err != nil {
				return err
			}
			receipt.BatchID, receipt.Revision = input.BatchID, input.ExpectedRevision+1
		case "move_item", "archive_item":
			// Lock batches before items in deterministic order, matching append/create.
			var existing itemRow
			found := tx.Raw("SELECT "+itemColumns+" FROM product_collection_items WHERE organization_id=? AND actor_id=? AND id=? AND archived_at IS NULL", scope.OrganizationID, scope.ActorID, input.ItemID).Scan(&existing)
			if found.Error != nil {
				return found.Error
			}
			if found.RowsAffected != 1 {
				return collection.ErrNotFound
			}
			batchIDs := []string{existing.BatchID}
			if input.TargetBatchID != "" && input.TargetBatchID != existing.BatchID {
				batchIDs = append(batchIDs, input.TargetBatchID)
			}
			if len(batchIDs) == 2 && batchIDs[0] > batchIDs[1] {
				batchIDs[0], batchIDs[1] = batchIDs[1], batchIDs[0]
			}
			for _, id := range batchIDs {
				if _, err := lockBatch(tx, scope, id, 0); err != nil {
					return err
				}
			}
			locked := tx.Raw("SELECT "+itemColumns+" FROM product_collection_items WHERE organization_id=? AND actor_id=? AND id=? AND archived_at IS NULL FOR UPDATE", scope.OrganizationID, scope.ActorID, input.ItemID).Scan(&existing)
			if locked.Error != nil {
				return locked.Error
			}
			if locked.RowsAffected != 1 {
				return collection.ErrNotFound
			}
			if existing.Revision != input.ExpectedRevision || existing.BatchID != batchIDs[0] && (len(batchIDs) < 2 || existing.BatchID != batchIDs[1]) {
				return collection.ErrConflict
			}
			if input.Action == "move_item" {
				if err := tx.Exec("UPDATE product_collection_items SET batch_id=?,revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", input.TargetBatchID, scope.OrganizationID, scope.ActorID, input.ItemID).Error; err != nil {
					return err
				}
				receipt.BatchID = input.TargetBatchID
			} else {
				if err := tx.Exec("UPDATE product_collection_items SET archived_at=?,revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", now, scope.OrganizationID, scope.ActorID, input.ItemID).Error; err != nil {
					return err
				}
				receipt.BatchID = existing.BatchID
			}
			for _, id := range batchIDs {
				if err := tx.Exec("UPDATE product_collection_batches SET revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, id).Error; err != nil {
					return err
				}
			}
			receipt.ItemID, receipt.Revision = input.ItemID, input.ExpectedRevision+1
		case "create_product", "add_acquisition":
			var source collection.Source
			if input.Action == "create_product" {
				if command.Envelope == nil {
					return collection.ErrInvalid
				}
				publisher, err := r.own(tx)
				if err != nil {
					return err
				}
				source, err = publisher.PublishOwn(ctx, scope, command.OperationID, *command.Envelope)
				if err != nil {
					return err
				}
			} else {
				if command.Source == nil {
					return collection.ErrInvalid
				}
				source = *command.Source
			}
			batchID := input.BatchID
			if batchID == "" {
				batchID = collection.StableID(scope.OrganizationID, scope.ActorID, "default", source.Kind, source.OperationID)
				name := "自有商品库"
				if source.Kind == "acquisition" {
					name = now.Format("01/02 15:04") + " · 1688商品采集"
				}
				if err := insertBatch(tx, scope, batchID, name, source.Kind, now); err != nil {
					return err
				}
			}
			if _, err := lockBatch(tx, scope, batchID, 0); err != nil {
				return err
			}
			itemID, err := appendSource(tx, scope, batchID, source, now)
			if err != nil {
				return err
			}
			receipt.BatchID, receipt.ItemID, receipt.Revision = batchID, itemID, 1
		default:
			return collection.ErrInvalid
		}
		raw, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		return tx.Exec("INSERT INTO product_collection_operations(organization_id,actor_id,member_id,id,command_key,input_hash,receipt_json,created_at) VALUES(?,?,?,?,?,?,?::jsonb,?)", command.Scope.OrganizationID, command.Scope.ActorID, command.Scope.MemberID, command.OperationID, command.Key, command.InputHash, string(raw), now).Error
	})
	if err != nil {
		return collection.Receipt{}, err
	}
	return receipt, nil
}
func (r *Repository) write(ctx context.Context, body func(*gorm.DB) error) (err error) {
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return collection.ErrUnavailable
	}
	committing := false
	defer func() {
		if recovered := recover(); recovered != nil {
			if !committing {
				_ = tx.Rollback().Error
			}
			panic(recovered)
		}
		if err != nil && !committing {
			_ = tx.Rollback().Error
		}
	}()
	if err = body(tx); err != nil {
		return err
	}
	committing = true
	if tx.Commit().Error != nil {
		return collection.ErrUnknown
	}
	if r.afterCommit != nil && r.afterCommit() != nil {
		return collection.ErrUnknown
	}
	return nil
}
func insertBatch(tx *gorm.DB, scope collection.Scope, id, name, kind string, now time.Time) error {
	return tx.Exec("INSERT INTO product_collection_batches(organization_id,actor_id,member_id,id,name,kind,revision,created_at) VALUES(?,?,?,?,?,?,1,?) ON CONFLICT DO NOTHING", scope.OrganizationID, scope.ActorID, scope.MemberID, id, name, kind, now).Error
}
func lockBatch(tx *gorm.DB, scope collection.Scope, id string, expected int64) (collection.Batch, error) {
	var batch collection.Batch
	row := tx.Raw("SELECT id,name,kind,revision,created_at,archived_at FROM product_collection_batches WHERE organization_id=? AND actor_id=? AND id=? AND archived_at IS NULL FOR UPDATE", scope.OrganizationID, scope.ActorID, id).Scan(&batch)
	if row.Error != nil {
		return batch, row.Error
	}
	if row.RowsAffected != 1 {
		return batch, collection.ErrNotFound
	}
	if expected > 0 && batch.Revision != expected {
		return batch, collection.ErrConflict
	}
	return batch, nil
}
func appendSource(tx *gorm.DB, scope collection.Scope, batchID string, source collection.Source, now time.Time) (string, error) {
	if source.ProductKey == "" || source.PublicationID == "" || source.Version == 0 {
		return "", collection.ErrInvalid
	}
	id := collection.StableID(scope.OrganizationID, scope.ActorID, source.PublicationID, "item")
	insert := tx.Exec("INSERT INTO product_collection_items(organization_id,actor_id,member_id,id,batch_id,product_key,publication_id,original_version,source_kind,source_operation_id,revision,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,1,?) ON CONFLICT DO NOTHING", scope.OrganizationID, scope.ActorID, scope.MemberID, id, batchID, source.ProductKey, source.PublicationID, source.Version, source.Kind, source.OperationID, now)
	if insert.Error != nil {
		return "", insert.Error
	}
	var existing itemRow
	row := tx.Raw("SELECT "+itemColumns+" FROM product_collection_items WHERE organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, id).Scan(&existing)
	if row.Error != nil {
		return "", row.Error
	}
	if row.RowsAffected != 1 || existing.ProductKey != source.ProductKey || existing.PublicationID != source.PublicationID || existing.OriginalVersion != source.Version {
		return "", collection.ErrConflict
	}
	if insert.RowsAffected == 1 {
		if err := tx.Exec("UPDATE product_collection_batches SET revision=revision+1 WHERE organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, batchID).Error; err != nil {
			return "", err
		}
	}
	return id, nil
}

// AppendPublished participates in the producer's transaction; no second commit or provider call.
func AppendPublished(ctx context.Context, tx *gorm.DB, scope collection.Scope, source collection.Source, publishedAt time.Time) error {
	if tx == nil || scope.Validate() != nil {
		return collection.ErrInvalid
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return collection.ErrUnavailable
	}
	batchID := collection.StableID(scope.OrganizationID, scope.ActorID, "default", source.Kind, source.OperationID)
	if err := insertBatch(tx.WithContext(ctx), scope, batchID, publishedAt.Format("01/02 15:04")+" · 1688商品采集", "acquisition", publishedAt); err != nil {
		return err
	}
	if _, err := lockBatch(tx, scope, batchID, 0); err != nil {
		return err
	}
	_, err := appendSource(tx, scope, batchID, source, publishedAt)
	return err
}

type itemRow struct {
	ID, BatchID, ProductKey, PublicationID, SourceKind, SourceOperationID string
	OriginalVersion                                                       uint64
	Revision                                                              int64
	CreatedAt                                                             time.Time
	ArchivedAt                                                            *time.Time
}

const itemColumns = "id,batch_id,product_key,publication_id,original_version,source_kind,source_operation_id,revision,created_at,archived_at"

func (row itemRow) item() collection.Item {
	return collection.Item{ID: row.ID, BatchID: row.BatchID, Source: collection.Source{ProductKey: row.ProductKey, PublicationID: row.PublicationID, Version: row.OriginalVersion, Kind: row.SourceKind, OperationID: row.SourceOperationID}, Revision: row.Revision, CreatedAt: row.CreatedAt, ArchivedAt: row.ArchivedAt}
}
func (r *Repository) ReadOperation(ctx context.Context, scope collection.Scope, operationID string) (collection.Receipt, error) {
	var stored struct{ ReceiptJSON []byte }
	row := r.db.WithContext(ctx).Raw("SELECT receipt_json FROM product_collection_operations WHERE organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, operationID).Scan(&stored)
	if row.Error != nil {
		return collection.Receipt{}, row.Error
	}
	if row.RowsAffected != 1 {
		return collection.Receipt{}, collection.ErrNotFound
	}
	var receipt collection.Receipt
	if json.Unmarshal(stored.ReceiptJSON, &receipt) != nil || receipt.OperationID != operationID {
		return receipt, collection.ErrUnavailable
	}
	receipt.Replayed = true
	return receipt, nil
}
func (r *Repository) ReadBatch(ctx context.Context, scope collection.Scope, id string) (collection.Batch, error) {
	var batch collection.Batch
	row := r.db.WithContext(ctx).Raw("SELECT id,name,kind,revision,created_at,archived_at FROM product_collection_batches WHERE organization_id=? AND actor_id=? AND id=? AND archived_at IS NULL", scope.OrganizationID, scope.ActorID, id).Scan(&batch)
	if row.Error != nil {
		return batch, row.Error
	}
	if row.RowsAffected != 1 {
		return batch, collection.ErrNotFound
	}
	return batch, nil
}
func likeKeyword(keyword string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(keyword) + "%"
}
func (r *Repository) ListBatches(ctx context.Context, scope collection.Scope, query collection.Query) (collection.Page[collection.Batch], error) {
	var page collection.Page[collection.Batch]
	page.Items = []collection.Batch{}
	base := r.db.WithContext(ctx).Table("product_collection_batches b").Where("b.organization_id=? AND b.actor_id=? AND b.archived_at IS NULL", scope.OrganizationID, scope.ActorID)
	if query.Keyword != "" {
		base = base.Where("b.name ILIKE ?", likeKeyword(query.Keyword))
	}
	if err := base.Count(&page.Total).Error; err != nil {
		return page, err
	}
	if query.After != "" {
		base = base.Where("b.id>?::uuid", query.After)
	}
	if err := base.Select("b.id,b.name,b.kind,b.revision,b.created_at,b.archived_at,(SELECT count(*) FROM product_collection_items i WHERE i.organization_id=b.organization_id AND i.actor_id=b.actor_id AND i.batch_id=b.id AND i.archived_at IS NULL) AS count").Order("b.id").Limit(query.Limit + 1).Find(&page.Items).Error; err != nil {
		return page, err
	}
	if len(page.Items) > query.Limit {
		page.Items = page.Items[:query.Limit]
		page.NextCursor = page.Items[len(page.Items)-1].ID
	}
	return page, nil
}
func (r *Repository) ListItems(ctx context.Context, scope collection.Scope, batchID string, query collection.Query) (collection.Page[collection.Item], error) {
	page := collection.Page[collection.Item]{Items: []collection.Item{}}
	base := r.db.WithContext(ctx).Table("product_collection_items").Where("organization_id=? AND actor_id=? AND batch_id=? AND archived_at IS NULL", scope.OrganizationID, scope.ActorID, batchID)
	if query.Keyword != "" {
		base = base.Where("product_key ILIKE ?", likeKeyword(query.Keyword))
	}
	if err := base.Count(&page.Total).Error; err != nil {
		return page, err
	}
	if query.After != "" {
		base = base.Where("id>?::uuid", query.After)
	}
	var rows []itemRow
	if err := base.Select(itemColumns).Order("id").Limit(query.Limit + 1).Find(&rows).Error; err != nil {
		return page, err
	}
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		page.Items = append(page.Items, row.item())
	}
	return page, nil
}
func (r *Repository) ReadItem(ctx context.Context, scope collection.Scope, id string) (collection.Item, error) {
	var row itemRow
	result := r.db.WithContext(ctx).Raw("SELECT "+itemColumns+" FROM product_collection_items WHERE organization_id=? AND actor_id=? AND id=? AND archived_at IS NULL", scope.OrganizationID, scope.ActorID, id).Scan(&row)
	if result.Error != nil {
		return collection.Item{}, result.Error
	}
	if result.RowsAffected != 1 {
		return collection.Item{}, collection.ErrNotFound
	}
	return row.item(), nil
}

var _ collection.Repository = (*Repository)(nil)
