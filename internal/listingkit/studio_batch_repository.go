package listingkit

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"task-processor/internal/listingkit/tenantctx"

	"gorm.io/gorm"
)

var ErrStudioBatchUnknownItemReference = errors.New("studio batch graph references unknown item")
var ErrStudioBatchOwnershipConflict = errors.New("studio batch update conflicts with immutable ownership")

type StudioBatchRepository interface {
	CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error
	ReplaceStudioBatchGenerationGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord) error
	CreateStudioBatchItems(ctx context.Context, batchID string, items []StudioBatchItemRecord) error
	CreateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error
	ClaimStudioBatchItem(ctx context.Context, itemID string, fromStatus StudioBatchItemStatus, toStatus StudioBatchItemStatus, updatedAt time.Time) (*StudioBatchItemRecord, bool, error)
	GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchRecord, error)
	GetStudioBatchItem(ctx context.Context, itemID string) (*StudioBatchItemRecord, error)
	GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error)
	ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error)
	ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []StudioMaterializedDesignRecord) error
	ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error
	UpdateStudioBatch(ctx context.Context, batch *StudioBatchRecord) error
	UpdateStudioBatchItem(ctx context.Context, item *StudioBatchItemRecord) error
	UpdateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error
	UpdateStudioMaterializedDesign(ctx context.Context, design *StudioMaterializedDesignRecord) error
}

type MemStudioBatchRepository struct {
	mu       sync.Mutex
	batches  map[string]StudioBatchRecord
	items    map[string]StudioBatchItemRecord
	attempts map[string]StudioGenerationAttemptRecord
	designs  map[string]StudioMaterializedDesignRecord
}

func NewMemStudioBatchRepository() *MemStudioBatchRepository {
	return &MemStudioBatchRepository{
		batches:  map[string]StudioBatchRecord{},
		items:    map[string]StudioBatchItemRecord{},
		attempts: map[string]StudioGenerationAttemptRecord{},
		designs:  map[string]StudioMaterializedDesignRecord{},
	}
}

func (r *MemStudioBatchRepository) CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error {
	if batch == nil {
		return nil
	}

	batchRow, itemRows, attemptRows, designRows, err := prepareStudioBatchGraph(ctx, batch, items, attempts, designs)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.batches[batchRow.ID] = batchRow
	for _, row := range itemRows {
		r.items[row.ID] = row
	}
	for _, row := range attemptRows {
		r.attempts[row.ID] = row
	}
	for _, row := range designRows {
		r.designs[row.ID] = row
	}
	return nil
}

func (r *MemStudioBatchRepository) ReplaceStudioBatchGenerationGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord) error {
	if batch == nil {
		return nil
	}

	batchRow, itemRows, _, _, err := prepareStudioBatchGraph(ctx, batch, items, nil, nil)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.batches[batchRow.ID]
	if ok && !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}

	r.batches[batchRow.ID] = batchRow
	itemIDs := make(map[string]struct{}, len(itemRows))
	for _, row := range itemRows {
		itemIDs[row.ID] = struct{}{}
		r.items[row.ID] = row
	}

	for itemID, item := range r.items {
		if item.BatchID == batchRow.ID {
			if _, keep := itemIDs[itemID]; !keep {
				delete(r.items, itemID)
			}
		}
	}
	for attemptID, attempt := range r.attempts {
		if attempt.BatchID == batchRow.ID {
			delete(r.attempts, attemptID)
		}
	}
	for designID, design := range r.designs {
		if design.BatchID == batchRow.ID {
			delete(r.designs, designID)
		}
	}
	return nil
}

func (r *MemStudioBatchRepository) CreateStudioBatchItems(ctx context.Context, batchID string, items []StudioBatchItemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	batch, ok := r.batches[batchID]
	if !ok || !matchesStudioBatchScope(ctx, batch.TenantID, batch.UserID) {
		return gorm.ErrRecordNotFound
	}

	for _, item := range items {
		row := item
		row.BatchID = batch.ID
		row.TenantID = batch.TenantID
		row.UserID = batch.UserID
		r.items[row.ID] = row
	}
	return nil
}

func (r *MemStudioBatchRepository) CreateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {
	if attempt == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[attempt.ItemID]
	if !ok || !matchesStudioBatchScope(ctx, item.TenantID, item.UserID) {
		return gorm.ErrRecordNotFound
	}

	row := *attempt
	row.BatchID = item.BatchID
	row.TenantID = item.TenantID
	row.UserID = item.UserID
	r.attempts[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) ClaimStudioBatchItem(ctx context.Context, itemID string, fromStatus StudioBatchItemStatus, toStatus StudioBatchItemStatus, updatedAt time.Time) (*StudioBatchItemRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[itemID]
	if !ok || !matchesStudioBatchScope(ctx, item.TenantID, item.UserID) {
		return nil, false, gorm.ErrRecordNotFound
	}
	if item.Status != fromStatus {
		cloned := item
		return &cloned, false, nil
	}

	item.Status = toStatus
	item.LastError = ""
	item.UpdatedAt = updatedAt
	r.items[itemID] = item
	cloned := item
	return &cloned, true, nil
}

func (r *MemStudioBatchRepository) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.batches[batchID]
	if !ok || !matchesStudioBatchScope(ctx, record.TenantID, record.UserID) {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := record
	return &cloned, nil
}

func (r *MemStudioBatchRepository) GetStudioBatchItem(ctx context.Context, itemID string) (*StudioBatchItemRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.items[itemID]
	if !ok || !matchesStudioBatchScope(ctx, record.TenantID, record.UserID) {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := record
	return &cloned, nil
}

func (r *MemStudioBatchRepository) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	batch, ok := r.batches[batchID]
	if !ok || !matchesStudioBatchScope(ctx, batch.TenantID, batch.UserID) {
		return nil, gorm.ErrRecordNotFound
	}

	return r.buildDetailGraphLocked(ctx, batch), nil
}

func (r *MemStudioBatchRepository) ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	batch, ok := r.batches[batchID]
	if !ok || !matchesStudioBatchScope(ctx, batch.TenantID, batch.UserID) {
		return nil, gorm.ErrRecordNotFound
	}

	records := make([]StudioMaterializedDesignRecord, 0, len(designIDs))
	for _, designID := range designIDs {
		row, ok := r.designs[designID]
		if !ok || row.BatchID != batchID || !matchesStudioBatchScope(ctx, row.TenantID, row.UserID) {
			continue
		}
		records = append(records, row)
	}
	sortStudioMaterializedDesigns(records)
	return records, nil
}

func (r *MemStudioBatchRepository) ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []StudioMaterializedDesignRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.items[itemID]
	if !ok || !matchesStudioBatchScope(ctx, item.TenantID, item.UserID) {
		return gorm.ErrRecordNotFound
	}

	for designID, row := range r.designs {
		if row.ItemID == itemID && matchesStudioBatchScope(ctx, row.TenantID, row.UserID) {
			delete(r.designs, designID)
		}
	}
	for _, design := range designs {
		row := design
		row.BatchID = item.BatchID
		row.ItemID = item.ID
		row.TenantID = item.TenantID
		row.UserID = item.UserID
		if row.TargetGroupKey == "" {
			row.TargetGroupKey = item.TargetGroupKey
		}
		if row.TargetGroupLabel == "" {
			row.TargetGroupLabel = item.TargetGroupLabel
		}
		if row.ReviewStatus == "" {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusUnreviewed
		}
		r.designs[row.ID] = row
	}
	return nil
}

func (r *MemStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	batch, ok := r.batches[batchID]
	if !ok || !matchesStudioBatchScope(ctx, batch.TenantID, batch.UserID) {
		return gorm.ErrRecordNotFound
	}

	approvedSet := make(map[string]struct{}, len(designIDs))
	for _, designID := range designIDs {
		approvedSet[designID] = struct{}{}
	}
	for _, designID := range designIDs {
		row, ok := r.designs[designID]
		if !ok || row.BatchID != batchID || !matchesStudioBatchScope(ctx, row.TenantID, row.UserID) {
			return gorm.ErrRecordNotFound
		}
	}

	for designID, row := range r.designs {
		if row.BatchID != batchID || !matchesStudioBatchScope(ctx, row.TenantID, row.UserID) {
			continue
		}
		if _, ok := approvedSet[designID]; ok {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusApproved
		} else {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusUnreviewed
		}
		row.UpdatedAt = updatedAt
		r.designs[designID] = row
	}
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioBatch(ctx context.Context, batch *StudioBatchRecord) error {
	if batch == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.batches[batch.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *batch
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	}
	r.batches[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioBatchItem(ctx context.Context, item *StudioBatchItemRecord) error {
	if item == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.items[item.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *item
	if row.BatchID == "" {
		row.BatchID = existing.BatchID
	} else if row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	} else if row.TenantID != existing.TenantID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	} else if row.UserID != existing.UserID {
		return ErrStudioBatchOwnershipConflict
	}
	r.items[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {
	if attempt == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.attempts[attempt.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *attempt
	if row.BatchID == "" {
		row.BatchID = existing.BatchID
	} else if row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ItemID == "" {
		row.ItemID = existing.ItemID
	} else if row.ItemID != existing.ItemID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	} else if row.TenantID != existing.TenantID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	} else if row.UserID != existing.UserID {
		return ErrStudioBatchOwnershipConflict
	}
	r.attempts[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) UpdateStudioMaterializedDesign(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	if design == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.designs[design.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *design
	if row.BatchID == "" {
		row.BatchID = existing.BatchID
	} else if row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ItemID == "" {
		row.ItemID = existing.ItemID
	} else if row.ItemID != existing.ItemID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.SourceAttemptID == "" {
		row.SourceAttemptID = existing.SourceAttemptID
	} else if row.SourceAttemptID != existing.SourceAttemptID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	} else if row.TenantID != existing.TenantID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	} else if row.UserID != existing.UserID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ReviewStatus == "" {
		row.ReviewStatus = existing.ReviewStatus
	}
	r.designs[row.ID] = row
	return nil
}

func (r *MemStudioBatchRepository) buildDetailGraphLocked(ctx context.Context, batch StudioBatchRecord) *StudioBatchDetailGraph {
	items := make([]StudioBatchItemRecord, 0)
	for _, item := range r.items {
		if item.BatchID != batch.ID || !matchesStudioBatchScope(ctx, item.TenantID, item.UserID) {
			continue
		}
		items = append(items, item)
	}
	sortStudioBatchItems(items)

	itemIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		itemIDs[item.ID] = struct{}{}
	}

	attemptsByItem := map[string][]StudioGenerationAttemptRecord{}
	for _, attempt := range r.attempts {
		if _, ok := itemIDs[attempt.ItemID]; !ok || !matchesStudioBatchScope(ctx, attempt.TenantID, attempt.UserID) {
			continue
		}
		attemptsByItem[attempt.ItemID] = append(attemptsByItem[attempt.ItemID], attempt)
	}
	for itemID := range attemptsByItem {
		sortStudioGenerationAttempts(attemptsByItem[itemID])
	}

	designsByItem := map[string][]StudioMaterializedDesignRecord{}
	for _, design := range r.designs {
		if _, ok := itemIDs[design.ItemID]; !ok || !matchesStudioBatchScope(ctx, design.TenantID, design.UserID) {
			continue
		}
		designsByItem[design.ItemID] = append(designsByItem[design.ItemID], design)
	}
	for itemID := range designsByItem {
		sortStudioMaterializedDesigns(designsByItem[itemID])
	}

	clonedBatch := batch
	return &StudioBatchDetailGraph{
		Batch:          &clonedBatch,
		Items:          items,
		AttemptsByItem: attemptsByItem,
		DesignsByItem:  designsByItem,
	}
}

type GormStudioBatchRepository struct {
	db *gorm.DB
}

func NewGormStudioBatchRepository(db *gorm.DB) *GormStudioBatchRepository {
	return &GormStudioBatchRepository{db: db}
}

func AutoMigrateStudioBatchRepository(db *gorm.DB) error {
	return db.AutoMigrate(
		&StudioBatchRecord{},
		&StudioBatchItemRecord{},
		&StudioGenerationAttemptRecord{},
		&StudioMaterializedDesignRecord{},
	)
}

func (r *GormStudioBatchRepository) CreateStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) error {
	if batch == nil {
		return nil
	}

	batchRow, itemRows, attemptRows, designRows, err := prepareStudioBatchGraph(ctx, batch, items, attempts, designs)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batchRow).Error; err != nil {
			return err
		}
		if len(itemRows) > 0 {
			if err := tx.Create(&itemRows).Error; err != nil {
				return err
			}
		}
		if len(attemptRows) > 0 {
			if err := tx.Create(&attemptRows).Error; err != nil {
				return err
			}
		}
		if len(designRows) > 0 {
			if err := tx.Create(&designRows).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormStudioBatchRepository) ReplaceStudioBatchGenerationGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord) error {
	if batch == nil {
		return nil
	}

	batchRow, itemRows, _, _, err := prepareStudioBatchGraph(ctx, batch, items, nil, nil)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing StudioBatchRecord
		findErr := applyStudioBatchAccessScope(tx, ctx).Where("id = ?", batchRow.ID).First(&existing).Error
		switch {
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			if err := tx.Create(&batchRow).Error; err != nil {
				return err
			}
		case findErr != nil:
			return findErr
		default:
			if err := applyStudioBatchAccessScope(tx, ctx).
				Model(&StudioBatchRecord{}).
				Where("id = ?", batchRow.ID).
				Updates(map[string]any{
					"status":                 batchRow.Status,
					"prompt":                 batchRow.Prompt,
					"grouped_image_mode":     batchRow.GroupedImageMode,
					"selection":              batchRow.Selection,
					"grouped_selections":     batchRow.GroupedSelections,
					"style_count":            batchRow.StyleCount,
					"variation_intensity":    batchRow.VariationIntensity,
					"artwork_model":          batchRow.ArtworkModel,
					"selected_sds_images":    batchRow.SelectedSDSImages,
					"transparent_background": batchRow.TransparentBackground,
					"shein_store_id":         batchRow.SheinStoreID,
					"updated_at":             batchRow.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}

		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("batch_id = ?", batchRow.ID).
			Delete(&StudioGenerationAttemptRecord{}).Error; err != nil {
			return err
		}
		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("batch_id = ?", batchRow.ID).
			Delete(&StudioMaterializedDesignRecord{}).Error; err != nil {
			return err
		}
		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("batch_id = ?", batchRow.ID).
			Delete(&StudioBatchItemRecord{}).Error; err != nil {
			return err
		}
		if len(itemRows) == 0 {
			return nil
		}
		return tx.Create(&itemRows).Error
	})
}

func (r *GormStudioBatchRepository) CreateStudioBatchItems(ctx context.Context, batchID string, items []StudioBatchItemRecord) error {
	batch, err := r.GetStudioBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	rows := make([]StudioBatchItemRecord, 0, len(items))
	for _, item := range items {
		row := item
		row.BatchID = batch.ID
		row.TenantID = batch.TenantID
		row.UserID = batch.UserID
		rows = append(rows, row)
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

func (r *GormStudioBatchRepository) CreateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {
	if attempt == nil {
		return nil
	}

	item, err := r.GetStudioBatchItem(ctx, attempt.ItemID)
	if err != nil {
		return err
	}
	row := *attempt
	row.BatchID = item.BatchID
	row.TenantID = item.TenantID
	row.UserID = item.UserID
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *GormStudioBatchRepository) ClaimStudioBatchItem(ctx context.Context, itemID string, fromStatus StudioBatchItemStatus, toStatus StudioBatchItemStatus, updatedAt time.Time) (*StudioBatchItemRecord, bool, error) {
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchItemRecord{}).
		Where("id = ? AND status = ?", itemID, fromStatus).
		Updates(map[string]any{
			"status":     toStatus,
			"last_error": "",
			"updated_at": updatedAt,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		item, err := r.GetStudioBatchItem(ctx, itemID)
		if err != nil {
			return nil, false, err
		}
		return item, false, nil
	}
	item, err := r.GetStudioBatchItem(ctx, itemID)
	if err != nil {
		return nil, false, err
	}
	return item, true, nil
}

func (r *GormStudioBatchRepository) GetStudioBatch(ctx context.Context, batchID string) (*StudioBatchRecord, error) {
	var record StudioBatchRecord
	err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", batchID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioBatchRepository) GetStudioBatchItem(ctx context.Context, itemID string) (*StudioBatchItemRecord, error) {
	var record StudioBatchItemRecord
	err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", itemID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *GormStudioBatchRepository) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetailGraph, error) {
	batch, err := r.GetStudioBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}

	var items []StudioBatchItemRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("batch_id = ?", batchID).
		Order("created_at ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}

	itemIDs := make([]string, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
	}

	attemptsByItem := map[string][]StudioGenerationAttemptRecord{}
	designsByItem := map[string][]StudioMaterializedDesignRecord{}
	if len(itemIDs) > 0 {
		var attempts []StudioGenerationAttemptRecord
		if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
			Where("item_id IN ?", itemIDs).
			Order("attempt_no ASC, id ASC").
			Find(&attempts).Error; err != nil {
			return nil, err
		}
		for _, attempt := range attempts {
			attemptsByItem[attempt.ItemID] = append(attemptsByItem[attempt.ItemID], attempt)
		}

		var designs []StudioMaterializedDesignRecord
		if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
			Where("item_id IN ?", itemIDs).
			Order("sort_order ASC, created_at ASC, id ASC").
			Find(&designs).Error; err != nil {
			return nil, err
		}
		for _, design := range designs {
			designsByItem[design.ItemID] = append(designsByItem[design.ItemID], design)
		}
	}

	return &StudioBatchDetailGraph{
		Batch:          batch,
		Items:          items,
		AttemptsByItem: attemptsByItem,
		DesignsByItem:  designsByItem,
	}, nil
}

func (r *GormStudioBatchRepository) ListStudioMaterializedDesignsByIDs(ctx context.Context, batchID string, designIDs []string) ([]StudioMaterializedDesignRecord, error) {
	if _, err := r.GetStudioBatch(ctx, batchID); err != nil {
		return nil, err
	}
	if len(designIDs) == 0 {
		return nil, nil
	}

	var designs []StudioMaterializedDesignRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("batch_id = ? AND id IN ?", batchID, designIDs).
		Order("sort_order ASC, created_at ASC, id ASC").
		Find(&designs).Error; err != nil {
		return nil, err
	}
	return designs, nil
}

func (r *GormStudioBatchRepository) ReplaceStudioItemMaterializedDesigns(ctx context.Context, itemID string, designs []StudioMaterializedDesignRecord) error {
	item, err := r.GetStudioBatchItem(ctx, itemID)
	if err != nil {
		return err
	}

	rows := make([]StudioMaterializedDesignRecord, 0, len(designs))
	for _, design := range designs {
		row := design
		row.BatchID = item.BatchID
		row.ItemID = item.ID
		row.TenantID = item.TenantID
		row.UserID = item.UserID
		if row.TargetGroupKey == "" {
			row.TargetGroupKey = item.TargetGroupKey
		}
		if row.TargetGroupLabel == "" {
			row.TargetGroupLabel = item.TargetGroupLabel
		}
		if row.ReviewStatus == "" {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusUnreviewed
		}
		rows = append(rows, row)
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := applyStudioBatchAccessScope(tx, ctx).
			Where("item_id = ?", itemID).
			Delete(&StudioMaterializedDesignRecord{}).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		return tx.Create(&rows).Error
	})
}

func (r *GormStudioBatchRepository) ReplaceStudioMaterializedDesignReviews(ctx context.Context, batchID string, designIDs []string, updatedAt time.Time) error {
	if _, err := r.GetStudioBatch(ctx, batchID); err != nil {
		return err
	}

	approvedIDs := append([]string(nil), designIDs...)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(approvedIDs) > 0 {
			var count int64
			if err := applyStudioBatchAccessScope(tx, ctx).
				Model(&StudioMaterializedDesignRecord{}).
				Where("batch_id = ? AND id IN ?", batchID, approvedIDs).
				Count(&count).Error; err != nil {
				return err
			}
			if count != int64(len(approvedIDs)) {
				return gorm.ErrRecordNotFound
			}
		}

		resetResult := applyStudioBatchAccessScope(tx, ctx).
			Model(&StudioMaterializedDesignRecord{}).
			Where("batch_id = ?", batchID).
			Updates(map[string]any{
				"review_status": StudioMaterializedDesignReviewStatusUnreviewed,
				"updated_at":    updatedAt,
			})
		if resetResult.Error != nil {
			return resetResult.Error
		}

		if len(approvedIDs) == 0 {
			return nil
		}

		approveResult := applyStudioBatchAccessScope(tx, ctx).
			Model(&StudioMaterializedDesignRecord{}).
			Where("batch_id = ? AND id IN ?", batchID, approvedIDs).
			Updates(map[string]any{
				"review_status": StudioMaterializedDesignReviewStatusApproved,
				"updated_at":    updatedAt,
			})
		return approveResult.Error
	})
}

func (r *GormStudioBatchRepository) UpdateStudioBatch(ctx context.Context, batch *StudioBatchRecord) error {
	if batch == nil {
		return nil
	}

	row := *batch
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"status":                 row.Status,
			"prompt":                 row.Prompt,
			"grouped_image_mode":     row.GroupedImageMode,
			"selection":              row.Selection,
			"grouped_selections":     row.GroupedSelections,
			"style_count":            row.StyleCount,
			"variation_intensity":    row.VariationIntensity,
			"artwork_model":          row.ArtworkModel,
			"selected_sds_images":    row.SelectedSDSImages,
			"transparent_background": row.TransparentBackground,
			"shein_store_id":         row.SheinStoreID,
			"updated_at":             row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchRepository) UpdateStudioBatchItem(ctx context.Context, item *StudioBatchItemRecord) error {
	if item == nil {
		return nil
	}

	row := *item
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	existing, err := r.GetStudioBatchItem(ctx, row.ID)
	if err != nil {
		return err
	}
	if row.BatchID != "" && row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioBatchItemRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"target_group_key":   row.TargetGroupKey,
			"target_group_label": row.TargetGroupLabel,
			"selection_ids":      row.SelectionIDs,
			"group_mode":         row.GroupMode,
			"status":             row.Status,
			"selection_count":    row.SelectionCount,
			"last_error":         row.LastError,
			"updated_at":         row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchRepository) UpdateStudioGenerationAttempt(ctx context.Context, attempt *StudioGenerationAttemptRecord) error {
	if attempt == nil {
		return nil
	}

	row := *attempt
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	var existing StudioGenerationAttemptRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", row.ID).
		First(&existing).Error; err != nil {
		return err
	}
	if row.BatchID != "" && row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ItemID != "" && row.ItemID != existing.ItemID {
		return ErrStudioBatchOwnershipConflict
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioGenerationAttemptRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"attempt_no":      row.AttemptNo,
			"status":          row.Status,
			"upstream_job_id": row.UpstreamJobID,
			"request_payload": row.RequestPayload,
			"result_payload":  row.ResultPayload,
			"error_message":   row.ErrorMessage,
			"started_at":      row.StartedAt,
			"finished_at":     row.FinishedAt,
			"updated_at":      row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormStudioBatchRepository) UpdateStudioMaterializedDesign(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	if design == nil {
		return nil
	}

	row := *design
	applyStudioBatchScopeDefaults(ctx, &row.TenantID, &row.UserID)
	var existing StudioMaterializedDesignRecord
	if err := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Where("id = ?", row.ID).
		First(&existing).Error; err != nil {
		return err
	}
	if row.BatchID != "" && row.BatchID != existing.BatchID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ItemID != "" && row.ItemID != existing.ItemID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.SourceAttemptID != "" && row.SourceAttemptID != existing.SourceAttemptID {
		return ErrStudioBatchOwnershipConflict
	}
	if row.ReviewStatus == "" {
		row.ReviewStatus = existing.ReviewStatus
	}
	result := applyStudioBatchAccessScope(r.db.WithContext(ctx), ctx).
		Model(&StudioMaterializedDesignRecord{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"target_group_key":   row.TargetGroupKey,
			"target_group_label": row.TargetGroupLabel,
			"image_url":          row.ImageURL,
			"review_status":      row.ReviewStatus,
			"sort_order":         row.SortOrder,
			"review_note":        row.ReviewNote,
			"updated_at":         row.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func prepareStudioBatchGraph(ctx context.Context, batch *StudioBatchRecord, items []StudioBatchItemRecord, attempts []StudioGenerationAttemptRecord, designs []StudioMaterializedDesignRecord) (StudioBatchRecord, []StudioBatchItemRecord, []StudioGenerationAttemptRecord, []StudioMaterializedDesignRecord, error) {
	batchRow := *batch
	applyStudioBatchScopeDefaults(ctx, &batchRow.TenantID, &batchRow.UserID)

	itemRows := make([]StudioBatchItemRecord, 0, len(items))
	itemIDs := make(map[string]struct{}, len(items))
	for _, item := range items {
		row := item
		row.BatchID = batchRow.ID
		row.TenantID = batchRow.TenantID
		row.UserID = batchRow.UserID
		itemRows = append(itemRows, row)
		itemIDs[row.ID] = struct{}{}
	}

	attemptRows := make([]StudioGenerationAttemptRecord, 0, len(attempts))
	for _, attempt := range attempts {
		if _, ok := itemIDs[attempt.ItemID]; !ok {
			return StudioBatchRecord{}, nil, nil, nil, ErrStudioBatchUnknownItemReference
		}
		row := attempt
		row.BatchID = batchRow.ID
		row.TenantID = batchRow.TenantID
		row.UserID = batchRow.UserID
		attemptRows = append(attemptRows, row)
	}

	designRows := make([]StudioMaterializedDesignRecord, 0, len(designs))
	for _, design := range designs {
		if _, ok := itemIDs[design.ItemID]; !ok {
			return StudioBatchRecord{}, nil, nil, nil, ErrStudioBatchUnknownItemReference
		}
		row := design
		row.BatchID = batchRow.ID
		row.TenantID = batchRow.TenantID
		row.UserID = batchRow.UserID
		if row.ReviewStatus == "" {
			row.ReviewStatus = StudioMaterializedDesignReviewStatusUnreviewed
		}
		designRows = append(designRows, row)
	}

	return batchRow, itemRows, attemptRows, designRows, nil
}

func applyStudioBatchScopeDefaults(ctx context.Context, tenantID *string, userID *string) {
	if tenantID != nil && *tenantID == "" {
		*tenantID = tenantctx.TenantIDFromContext(ctx)
	}
	if userID != nil && *userID == "" {
		*userID = RequestUserIDFromContext(ctx)
	}
}

func applyStudioBatchAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	tenantID, ok := tenantctx.TenantScopeFromContext(ctx)
	if ok {
		if tenantID == tenantctx.DefaultTenantID {
			db = db.Where("(tenant_id = ? OR tenant_id = '' OR tenant_id IS NULL)", tenantID)
		} else {
			db = db.Where("tenant_id = ?", tenantID)
		}
	}
	if OwnerScopeEnabled() {
		if userID := RequestUserIDFromContext(ctx); userID != "" {
			db = db.Where("user_id = ?", userID)
		}
	}
	return db
}

func matchesStudioBatchScope(ctx context.Context, tenantID string, userID string) bool {
	if !tenantctx.MatchesTenant(tenantID, tenantctx.TenantIDFromContext(ctx)) {
		return false
	}
	if !OwnerScopeEnabled() {
		return true
	}
	requestUserID := RequestUserIDFromContext(ctx)
	return requestUserID == "" || requestUserID == userID
}

func sortStudioBatchItems(items []StudioBatchItemRecord) {
	slices.SortStableFunc(items, func(a, b StudioBatchItemRecord) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}

func sortStudioGenerationAttempts(attempts []StudioGenerationAttemptRecord) {
	slices.SortStableFunc(attempts, func(a, b StudioGenerationAttemptRecord) int {
		if a.AttemptNo < b.AttemptNo {
			return -1
		}
		if a.AttemptNo > b.AttemptNo {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}

func sortStudioMaterializedDesigns(designs []StudioMaterializedDesignRecord) {
	slices.SortStableFunc(designs, func(a, b StudioMaterializedDesignRecord) int {
		if a.SortOrder < b.SortOrder {
			return -1
		}
		if a.SortOrder > b.SortOrder {
			return 1
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
}
