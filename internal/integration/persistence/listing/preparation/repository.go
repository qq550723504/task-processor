package preparationpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"strings"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if VerifySchema(ctx, db) != nil {
		return nil, preparation.ErrUnavailable
	}
	return &Repository{db}, nil
}

type preparationRow struct {
	OrganizationID, ActorID, MemberID string
	ID, CommandKey, InputHash         string
	InputJSON                         []byte
	SourceBatchID                     string
	SourceRevision                    int64
	Name                              string
	ItemCount, Revision               int64
	CreatedAt                         time.Time
}

func (preparationRow) TableName() string { return "listing_preparations" }
func (r preparationRow) value() preparation.Preparation {
	return preparation.Preparation{ID: r.ID, SourceBatchID: r.SourceBatchID, SourceRevision: r.SourceRevision, Name: r.Name, Count: r.ItemCount, Revision: r.Revision, CreatedAt: r.CreatedAt.UTC()}
}

type sourceRow struct {
	OrganizationID, ActorID, MemberID   string
	ID, PreparationID, CollectionItemID string
	CollectionRevision                  int64
	ProductKey, PublicationID           string
	OriginalVersion                     uint64
	SourceKind, SourceOperationID       string
}

func (sourceRow) TableName() string { return "listing_preparation_sources" }
func (r sourceRow) value() preparation.SourceItem {
	return preparation.SourceItem{ID: r.ID, PreparationID: r.PreparationID, CollectionItemID: r.CollectionItemID, CollectionRevision: r.CollectionRevision, Source: collection.Source{ProductKey: r.ProductKey, PublicationID: r.PublicationID, Version: r.OriginalVersion, Kind: r.SourceKind, OperationID: r.SourceOperationID}}
}

func readTransfer(db *gorm.DB, scope preparation.Scope, key string) (preparationRow, error) {
	if scope.Validate() != nil || !collection.ValidID(key) {
		return preparationRow{}, preparation.ErrInvalid
	}
	var row preparationRow
	result := db.Where("organization_id=? AND actor_id=? AND command_key=?", scope.OrganizationID, scope.ActorID, key).Take(&row)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return row, preparation.ErrNotFound
	}
	if result.Error != nil {
		return row, preparation.ErrUnavailable
	}
	var saved preparation.TransferInput
	if row.ID != preparation.OperationID(scope, key) || row.ItemCount < 1 || row.Revision < 1 || row.MemberID != scope.MemberID || json.Unmarshal(row.InputJSON, &saved) != nil || saved.Validate() != nil || row.InputHash != collection.Digest(saved) {
		return row, preparation.ErrUnavailable
	}
	return row, nil
}

func (r *Repository) FindTransfer(ctx context.Context, scope preparation.Scope, key string, input preparation.TransferInput) (preparation.TransferReceipt, error) {
	row, err := readTransfer(r.db.WithContext(ctx), scope, key)
	if err != nil {
		return preparation.TransferReceipt{}, err
	}
	// Hash JSON after decoding, since PostgreSQL jsonb reorders object keys.
	var saved preparation.TransferInput
	if json.Unmarshal(row.InputJSON, &saved) != nil {
		return preparation.TransferReceipt{}, preparation.ErrUnavailable
	}
	if row.InputHash != collection.Digest(input) || collection.Digest(saved) != row.InputHash {
		return preparation.TransferReceipt{}, preparation.ErrConflict
	}
	return preparation.TransferReceipt{Preparation: row.value(), Replayed: true}, nil
}

func (r *Repository) ReadByKey(ctx context.Context, scope preparation.Scope, key string) (preparation.TransferReceipt, error) {
	row, err := readTransfer(r.db.WithContext(ctx), scope, key)
	if err != nil {
		return preparation.TransferReceipt{}, err
	}
	return preparation.TransferReceipt{Preparation: row.value(), Replayed: true}, nil
}

func (r *Repository) Transfer(ctx context.Context, commit preparation.TransferCommit) (preparation.TransferReceipt, error) {
	scope, key, input, proof, err := commit.Read(ctx)
	if err != nil {
		return preparation.TransferReceipt{}, err
	}
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return preparation.TransferReceipt{}, preparation.ErrUnavailable
	}
	defer tx.Rollback()
	if tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", collection.Digest([]string{scope.OrganizationID, scope.ActorID, "supply-transfer", key})).Error != nil {
		return preparation.TransferReceipt{}, preparation.ErrUnavailable
	}
	if existing, lookupErr := readTransfer(tx, scope, key); lookupErr == nil {
		if existing.InputHash != collection.Digest(input) {
			return preparation.TransferReceipt{}, preparation.ErrConflict
		}
		return preparation.TransferReceipt{Preparation: existing.value(), Replayed: true}, nil
	} else if !errors.Is(lookupErr, preparation.ErrNotFound) {
		return preparation.TransferReceipt{}, lookupErr
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return preparation.TransferReceipt{}, preparation.ErrInvalid
	}
	row := preparationRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ID: preparation.OperationID(scope, key), CommandKey: key, InputHash: collection.Digest(input), InputJSON: raw, SourceBatchID: input.BatchID, SourceRevision: input.ExpectedRevision, Name: "", Revision: 1, CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if tx.Create(&row).Error != nil {
		return preparation.TransferReceipt{}, preparation.ErrUnavailable
	}
	reader, err := collectionstore.NewSelectionReader(tx)
	if err != nil {
		return preparation.TransferReceipt{}, err
	}
	buffer := make([]sourceRow, 0, 100)
	flush := func() error {
		if len(buffer) == 0 {
			return nil
		}
		if tx.Create(&buffer).Error != nil {
			return preparation.ErrUnavailable
		}
		buffer = buffer[:0]
		return nil
	}
	count, err := reader.VisitSelection(ctx, proof, func(item collection.Item) error {
		// A blank SDS template is input to customization, not a finished supply product.
		if item.Source.Kind == "sds_template" {
			return preparation.ErrInvalid
		}
		buffer = append(buffer, sourceRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ID: collection.StableID(row.ID, item.ID), PreparationID: row.ID, CollectionItemID: item.ID, CollectionRevision: item.Revision, ProductKey: item.Source.ProductKey, PublicationID: item.Source.PublicationID, OriginalVersion: item.Source.Version, SourceKind: item.Source.Kind, SourceOperationID: item.Source.OperationID})
		if len(buffer) == 100 {
			return flush()
		}
		return nil
	})
	if err != nil {
		return preparation.TransferReceipt{}, err
	}
	if err = flush(); err != nil {
		return preparation.TransferReceipt{}, err
	}
	var name string
	if tx.Raw("SELECT name FROM product_collection_batches WHERE organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, input.BatchID).Scan(&name).Error != nil || name == "" {
		return preparation.TransferReceipt{}, preparation.ErrUnavailable
	}
	row.Name, row.ItemCount = name, count
	if tx.Model(&preparationRow{}).Where("organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, row.ID).Updates(map[string]any{"name": name, "item_count": count}).Error != nil {
		return preparation.TransferReceipt{}, preparation.ErrUnavailable
	}
	if _, _, _, _, err = commit.Read(ctx); err != nil {
		return preparation.TransferReceipt{}, err
	}
	if tx.Commit().Error != nil {
		return preparation.TransferReceipt{}, preparation.ErrUnknown
	}
	return preparation.TransferReceipt{Preparation: row.value()}, nil
}

func keyword(value string) string {
	return "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value) + "%"
}

func (r *Repository) Read(ctx context.Context, scope preparation.Scope, id string) (preparation.Preparation, error) {
	if scope.Validate() != nil || !collection.ValidID(id) {
		return preparation.Preparation{}, preparation.ErrInvalid
	}
	var row preparationRow
	err := r.db.WithContext(ctx).Where("organization_id=? AND actor_id=? AND member_id=? AND id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Take(&row).Error
	if err != nil {
		return preparation.Preparation{}, operationError(err)
	}
	if row.ItemCount < 1 {
		return preparation.Preparation{}, preparation.ErrUnavailable
	}
	return row.value(), nil
}
func (r *Repository) List(ctx context.Context, scope preparation.Scope, query preparation.Query) (collection.Page[preparation.Preparation], error) {
	page := collection.Page[preparation.Preparation]{Items: []preparation.Preparation{}}
	if scope.Validate() != nil || query.Validate() != nil {
		return page, preparation.ErrInvalid
	}
	base := r.db.WithContext(ctx).Model(&preparationRow{}).Where("organization_id=? AND actor_id=? AND member_id=?", scope.OrganizationID, scope.ActorID, scope.MemberID)
	if query.Keyword != "" {
		base = base.Where("name ILIKE ?", keyword(query.Keyword))
	}
	if base.Count(&page.Total).Error != nil {
		return page, preparation.ErrUnavailable
	}
	if query.After != "" {
		base = base.Where("id>?::uuid", query.After)
	}
	var rows []preparationRow
	if base.Order("id").Limit(query.Limit+1).Find(&rows).Error != nil {
		return page, preparation.ErrUnavailable
	}
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		if row.ItemCount < 1 {
			return page, preparation.ErrUnavailable
		}
		page.Items = append(page.Items, row.value())
	}
	return page, nil
}
func (r *Repository) ListSources(ctx context.Context, scope preparation.Scope, id string, query preparation.Query) (collection.Page[preparation.SourceItem], error) {
	page := collection.Page[preparation.SourceItem]{Items: []preparation.SourceItem{}}
	if scope.Validate() != nil || !collection.ValidID(id) || query.Validate() != nil {
		return page, preparation.ErrInvalid
	}
	var parent preparationRow
	result := r.db.WithContext(ctx).Where("organization_id=? AND actor_id=? AND member_id=? AND id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, id).Take(&parent)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return page, preparation.ErrNotFound
	}
	if result.Error != nil {
		return page, preparation.ErrUnavailable
	}
	base := r.db.WithContext(ctx).Model(&sourceRow{}).Where("organization_id=? AND actor_id=? AND member_id=? AND preparation_id=?", scope.OrganizationID, scope.ActorID, scope.MemberID, id)
	if query.Keyword != "" {
		base = base.Where("product_key ILIKE ?", keyword(query.Keyword))
	}
	if base.Count(&page.Total).Error != nil {
		return page, preparation.ErrUnavailable
	}
	if query.After != "" {
		base = base.Where("id>?::uuid", query.After)
	}
	var rows []sourceRow
	if base.Order("id").Limit(query.Limit+1).Find(&rows).Error != nil {
		return page, preparation.ErrUnavailable
	}
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		page.NextCursor = rows[len(rows)-1].ID
	}
	for _, row := range rows {
		page.Items = append(page.Items, row.value())
	}
	return page, nil
}

var _ preparation.Repository = (*Repository)(nil)
