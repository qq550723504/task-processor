package preparationpersistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/authidentity"
	"task-processor/internal/listing/preparation"
	"task-processor/internal/product/collection"
)

type OperationRepository struct{ db *gorm.DB }

func NewOperationRepository(ctx context.Context, db *gorm.DB) (*OperationRepository, error) {
	if VerifyOperationSchema(ctx, db) != nil {
		return nil, preparation.ErrUnavailable
	}
	return &OperationRepository{db: db}, nil
}

type operationRow struct {
	OrganizationID, ActorID, MemberID, ID, CommandKey, InputHash string
	InputJSON                                                    []byte
	PreparationID, StoreID, Action                               string
	ItemCount, CompletedCount                                    int64
	Status, Execution                                            string
	CreatedAt                                                    time.Time
}

func (operationRow) TableName() string { return "listing_preparation_operations" }
func (r operationRow) value() (preparation.Operation, error) {
	var input preparation.OperationInput
	scope := collection.Scope{OrganizationID: r.OrganizationID, ActorID: r.ActorID, MemberID: r.MemberID}
	if scope.Validate() != nil || json.Unmarshal(r.InputJSON, &input) != nil || input.Validate() != nil || collection.Digest(input) != r.InputHash || r.ID != preparation.OperationCommandID(scope, r.CommandKey) || input.Action != r.Action || input.PreparationID != r.PreparationID || input.StoreID != r.StoreID || r.ItemCount < 1 || r.CompletedCount < 0 || r.CompletedCount > r.ItemCount {
		return preparation.Operation{}, preparation.ErrUnavailable
	}
	if r.Status != preparation.OperationPending && r.Status != preparation.OperationRunning && r.Status != preparation.OperationCompleted && r.Status != preparation.OperationCancelled {
		return preparation.Operation{}, preparation.ErrUnavailable
	}
	if r.Execution != "pending" && r.Execution != "started" && r.Execution != "start_unknown" {
		return preparation.Operation{}, preparation.ErrUnavailable
	}
	return preparation.Operation{ID: r.ID, Owner: scope, Input: input, Count: r.ItemCount, Completed: r.CompletedCount, Status: r.Status, Execution: r.Execution, CreatedAt: r.CreatedAt.UTC()}, nil
}

type operationItemRow struct {
	OrganizationID, ActorID, MemberID, OperationID, SourceID string
	RecordID                                                 *string
	RecordRevision                                           *int64
	Status, ResultReference, Note                            string
}

func (operationItemRow) TableName() string { return "listing_preparation_operation_items" }
func (r operationItemRow) value() preparation.OperationItem {
	value := preparation.OperationItem{SourceID: r.SourceID, Status: r.Status, ResultReference: r.ResultReference, Note: r.Note}
	if r.RecordID != nil {
		value.RecordID = *r.RecordID
	}
	if r.RecordRevision != nil {
		value.RecordRevision = *r.RecordRevision
	}
	return value
}
func operationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return preparation.ErrNotFound
	}
	for _, known := range []error{preparation.ErrNotFound, preparation.ErrConflict, preparation.ErrForbidden, preparation.ErrInvalid, preparation.ErrUnavailable} {
		if errors.Is(err, known) {
			return known
		}
	}
	return preparation.ErrUnavailable
}
func operationBase(db *gorm.DB, scope collection.Scope) *gorm.DB {
	return db.Where("organization_id=? AND actor_id=? AND member_id=?", scope.OrganizationID, scope.ActorID, scope.MemberID)
}
func readOperation(db *gorm.DB, scope collection.Scope, id string) (operationRow, error) {
	if scope.Validate() != nil || !collection.ValidID(id) {
		return operationRow{}, preparation.ErrInvalid
	}
	var row operationRow
	err := operationBase(db, scope).Where("id=?", id).Take(&row).Error
	if err != nil {
		return row, operationError(err)
	}
	_, err = row.value()
	return row, err
}
func (r *OperationRepository) FindOperation(ctx context.Context, scope collection.Scope, key, hash string) (preparation.OperationReceipt, error) {
	if scope.Validate() != nil || !collection.ValidID(key) || len(hash) != 64 {
		return preparation.OperationReceipt{}, preparation.ErrInvalid
	}
	row, err := readOperation(r.db.WithContext(ctx), scope, preparation.OperationCommandID(scope, key))
	if err != nil {
		return preparation.OperationReceipt{}, err
	}
	if row.InputHash != hash {
		return preparation.OperationReceipt{}, preparation.ErrConflict
	}
	value, err := row.value()
	return preparation.OperationReceipt{Operation: value, Replayed: true}, err
}
func (r *OperationRepository) PrepareOperation(ctx context.Context, proof preparation.OperationCommit) (preparation.OperationReceipt, error) {
	scope, key, input, err := proof.Read(ctx)
	if err != nil {
		return preparation.OperationReceipt{}, err
	}
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return preparation.OperationReceipt{}, preparation.ErrUnavailable
	}
	defer tx.Rollback()
	if tx.Exec("SET LOCAL synchronous_commit=on").Error != nil || tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", collection.Digest([]string{scope.OrganizationID, scope.ActorID, "supply-operation", key})).Error != nil {
		return preparation.OperationReceipt{}, preparation.ErrUnavailable
	}
	row, err := readOperation(tx, scope, preparation.OperationCommandID(scope, key))
	if err == nil {
		if row.InputHash != collection.Digest(input) {
			return preparation.OperationReceipt{}, preparation.ErrConflict
		}
		value, err := row.value()
		return preparation.OperationReceipt{Operation: value, Replayed: true}, err
	}
	if !errors.Is(err, preparation.ErrNotFound) {
		return preparation.OperationReceipt{}, err
	}
	var parent preparationRow
	if err := operationBase(tx, scope).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", input.PreparationID).Take(&parent).Error; err != nil {
		return preparation.OperationReceipt{}, operationError(err)
	}
	if parent.Revision != input.ExpectedRevision {
		return preparation.OperationReceipt{}, preparation.ErrConflict
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return preparation.OperationReceipt{}, preparation.ErrInvalid
	}
	row = operationRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, ID: preparation.OperationCommandID(scope, key), CommandKey: key, InputHash: collection.Digest(input), InputJSON: raw, PreparationID: input.PreparationID, StoreID: input.StoreID, Action: input.Action, ItemCount: parent.ItemCount, Status: preparation.OperationPending, Execution: "pending", CreatedAt: time.Now().UTC().Truncate(time.Microsecond)}
	if len(input.SourceIDs) > 0 {
		row.ItemCount = int64(len(input.SourceIDs))
	}
	if row.ItemCount < 1 {
		return preparation.OperationReceipt{}, preparation.ErrConflict
	}
	if tx.Create(&row).Error != nil {
		return preparation.OperationReceipt{}, preparation.ErrUnavailable
	}
	var copied int64
	query := operationBase(tx.Model(&sourceRow{}), scope).Where("preparation_id=?", input.PreparationID)
	if len(input.SourceIDs) > 0 {
		query = query.Where("id IN ?", input.SourceIDs)
	}
	// Membership is immutable, but source locks also order this transaction
	// with future owner commands. Traverse every page without UI truncation.
	after := ""
	for {
		var sources []sourceRow
		page := query.Session(&gorm.Session{})
		if after != "" {
			page = page.Where("id>?::uuid", after)
		}
		if page.Clauses(clause.Locking{Strength: "SHARE"}).Order("id").Limit(100).Find(&sources).Error != nil {
			return preparation.OperationReceipt{}, preparation.ErrUnavailable
		}
		if len(sources) == 0 {
			break
		}
		items := make([]operationItemRow, 0, len(sources))
		for _, source := range sources {
			item := operationItemRow{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: scope.MemberID, OperationID: row.ID, SourceID: source.ID, Status: preparation.ItemPending}
			if input.Action == preparation.OperationUpload {
				var target struct {
					CurrentRecordID string
					Revision        int64
				}
				result := tx.Table("listing_preparation_targets").Clauses(clause.Locking{Strength: "SHARE"}).Where("organization_id=? AND actor_id=? AND member_id=? AND source_id=? AND store_id=? AND site='shein-us'", scope.OrganizationID, scope.ActorID, scope.MemberID, source.ID, input.StoreID).Take(&target)
				if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
					return preparation.OperationReceipt{}, preparation.ErrUnavailable
				}
				if result.Error == nil {
					item.RecordID, item.RecordRevision = &target.CurrentRecordID, &target.Revision
				} else {
					item.Status, item.Note = preparation.ItemMissing, "尚无该店铺的适配资料"
					row.CompletedCount++
				}
			}
			items = append(items, item)
		}
		if tx.Create(&items).Error != nil {
			return preparation.OperationReceipt{}, preparation.ErrUnavailable
		}
		copied += int64(len(items))
		after = sources[len(sources)-1].ID
	}
	if copied != row.ItemCount {
		return preparation.OperationReceipt{}, preparation.ErrConflict
	}
	if row.CompletedCount == row.ItemCount {
		row.Status = preparation.OperationCompleted
	}
	if tx.Model(&operationRow{}).Where("organization_id=? AND actor_id=? AND id=?", scope.OrganizationID, scope.ActorID, row.ID).Updates(map[string]any{"completed_count": row.CompletedCount, "status": row.Status}).Error != nil {
		return preparation.OperationReceipt{}, preparation.ErrUnavailable
	}
	if _, _, _, err = proof.Read(ctx); err != nil {
		return preparation.OperationReceipt{}, err
	}
	if tx.Commit().Error != nil {
		return preparation.OperationReceipt{}, preparation.ErrUnknown
	}
	value, err := row.value()
	return preparation.OperationReceipt{Operation: value}, err
}
func (r *OperationRepository) ReadOperation(ctx context.Context, scope collection.Scope, id string) (preparation.Operation, error) {
	row, err := readOperation(r.db.WithContext(ctx), scope, id)
	if err != nil {
		return preparation.Operation{}, err
	}
	return row.value()
}
func (r *OperationRepository) ReadExecutionOperation(ctx context.Context, org, id string) (preparation.Operation, error) {
	if !authidentity.IsBoundedIdentifier(org) || !collection.ValidID(id) {
		return preparation.Operation{}, preparation.ErrInvalid
	}
	var row operationRow
	if err := r.db.WithContext(ctx).Where("organization_id=? AND id=?", org, id).Take(&row).Error; err != nil {
		return preparation.Operation{}, operationError(err)
	}
	return row.value()
}
func (r *OperationRepository) ListOperationItems(ctx context.Context, scope collection.Scope, id string, q collection.Query) (collection.Page[preparation.OperationItem], error) {
	page := collection.Page[preparation.OperationItem]{Items: []preparation.OperationItem{}}
	if q.Validate() != nil || q.Keyword != "" {
		return page, preparation.ErrInvalid
	}
	if _, err := r.ReadOperation(ctx, scope, id); err != nil {
		return page, err
	}
	base := operationBase(r.db.WithContext(ctx).Model(&operationItemRow{}), scope).Where("operation_id=?", id)
	if base.Count(&page.Total).Error != nil {
		return page, preparation.ErrUnavailable
	}
	if q.After != "" {
		base = base.Where("source_id>?::uuid", q.After)
	}
	var rows []operationItemRow
	if base.Order("source_id").Limit(q.Limit+1).Find(&rows).Error != nil {
		return page, preparation.ErrUnavailable
	}
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		page.NextCursor = rows[len(rows)-1].SourceID
	}
	for _, row := range rows {
		if !preparation.ValidItemStatus(row.Status) {
			return page, preparation.ErrUnavailable
		}
		page.Items = append(page.Items, row.value())
	}
	return page, nil
}
func (r *OperationRepository) mutate(ctx context.Context, proof preparation.OperationAccess, fn func(*gorm.DB, *operationRow) error) error {
	operation, err := proof.Read(ctx)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := readOperation(tx.Clauses(clause.Locking{Strength: "UPDATE"}), operation.Owner, operation.ID)
		if err != nil {
			return err
		}
		if row.InputHash != collection.Digest(operation.Input) {
			return preparation.ErrConflict
		}
		if err = fn(tx, &row); err != nil {
			return err
		}
		_, err = proof.Read(ctx)
		return err
	})
	return operationError(err)
}
func (r *OperationRepository) CancelOperation(ctx context.Context, proof preparation.OperationAccess) (preparation.Operation, error) {
	var output preparation.Operation
	err := r.mutate(ctx, proof, func(tx *gorm.DB, row *operationRow) error {
		if row.Status == preparation.OperationCompleted || row.Status == preparation.OperationCancelled {
			var err error
			output, err = row.value()
			return err
		}
		result := tx.Model(&operationItemRow{}).Where("organization_id=? AND actor_id=? AND operation_id=? AND status='pending'", row.OrganizationID, row.ActorID, row.ID).Updates(map[string]any{"status": preparation.ItemCancelled, "note": "已取消未开始项"})
		if result.Error != nil {
			return result.Error
		}
		row.CompletedCount += result.RowsAffected
		row.Status = preparation.OperationCancelled
		if err := tx.Model(&operationRow{}).Where("organization_id=? AND actor_id=? AND id=?", row.OrganizationID, row.ActorID, row.ID).Updates(map[string]any{"status": row.Status, "completed_count": row.CompletedCount}).Error; err != nil {
			return err
		}
		var err error
		output, err = row.value()
		return err
	})
	return output, err
}
func (r *OperationRepository) MarkExecution(ctx context.Context, proof preparation.OperationAccess, status string) error {
	if status != "started" && status != "start_unknown" {
		return preparation.ErrInvalid
	}
	return r.mutate(ctx, proof, func(tx *gorm.DB, row *operationRow) error {
		if row.Execution == "started" || row.Status == preparation.OperationCompleted || row.Status == preparation.OperationCancelled {
			return nil
		}
		return tx.Model(&operationRow{}).Where("organization_id=? AND actor_id=? AND id=?", row.OrganizationID, row.ActorID, row.ID).Update("execution", status).Error
	})
}
func (r *OperationRepository) BeginOperationItem(ctx context.Context, proof preparation.OperationAccess, id string) (preparation.OperationItem, error) {
	var output preparation.OperationItem
	if !collection.ValidID(id) {
		return output, preparation.ErrInvalid
	}
	err := r.mutate(ctx, proof, func(tx *gorm.DB, row *operationRow) error {
		var item operationItemRow
		if err := tx.Where("organization_id=? AND actor_id=? AND operation_id=? AND source_id=?", row.OrganizationID, row.ActorID, row.ID, id).Take(&item).Error; err != nil {
			return err
		}
		if preparation.ItemTerminal(item.Status) {
			output = item.value()
			return nil
		}
		if row.Status == preparation.OperationCancelled && item.Status != preparation.ItemRunning {
			return preparation.ErrConflict
		}
		if item.Status == preparation.ItemPending {
			item.Status = preparation.ItemRunning
			if err := tx.Model(&operationItemRow{}).Where("organization_id=? AND actor_id=? AND operation_id=? AND source_id=? AND status='pending'", row.OrganizationID, row.ActorID, row.ID, id).Update("status", item.Status).Error; err != nil {
				return err
			}
			if err := tx.Model(&operationRow{}).Where("organization_id=? AND actor_id=? AND id=?", row.OrganizationID, row.ActorID, row.ID).Update("status", preparation.OperationRunning).Error; err != nil {
				return err
			}
		}
		output = item.value()
		return nil
	})
	return output, err
}
func (r *OperationRepository) FinishOperationItem(ctx context.Context, proof preparation.OperationAccess, value preparation.OperationItem) error {
	if !collection.ValidID(value.SourceID) || !preparation.ItemTerminal(value.Status) || utf8.RuneCountInString(value.Note) > 300 || !utf8.ValidString(value.Note) || value.ResultReference != "" && !authidentity.IsBoundedIdentifier(value.ResultReference) {
		return preparation.ErrInvalid
	}
	return r.mutate(ctx, proof, func(tx *gorm.DB, row *operationRow) error {
		var item operationItemRow
		if err := tx.Where("organization_id=? AND actor_id=? AND operation_id=? AND source_id=?", row.OrganizationID, row.ActorID, row.ID, value.SourceID).Take(&item).Error; err != nil {
			return err
		}
		if preparation.ItemTerminal(item.Status) {
			prior := item.value()
			if prior.Status == value.Status && prior.ResultReference == value.ResultReference && prior.Note == value.Note {
				return nil
			}
			return preparation.ErrConflict
		}
		if item.Status != preparation.ItemRunning || item.value().RecordID != value.RecordID || item.value().RecordRevision != value.RecordRevision {
			return preparation.ErrConflict
		}
		if err := tx.Model(&operationItemRow{}).Where("organization_id=? AND actor_id=? AND operation_id=? AND source_id=?", row.OrganizationID, row.ActorID, row.ID, value.SourceID).Updates(map[string]any{"status": value.Status, "result_reference": value.ResultReference, "note": value.Note}).Error; err != nil {
			return err
		}
		row.CompletedCount++
		if row.CompletedCount == row.ItemCount && row.Status != preparation.OperationCancelled {
			row.Status = preparation.OperationCompleted
		}
		return tx.Model(&operationRow{}).Where("organization_id=? AND actor_id=? AND id=?", row.OrganizationID, row.ActorID, row.ID).Updates(map[string]any{"completed_count": row.CompletedCount, "status": row.Status}).Error
	})
}

var _ preparation.OperationRepository = (*OperationRepository)(nil)
