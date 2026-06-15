package listingkit

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

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
			row.ReviewStatus = StudioMaterializedDesignReviewStatusApproved
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
	applyStudioBatchDefaultScope(existing.TenantID, existing.UserID, &row.TenantID, &row.UserID)
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
	if err := resolveStudioBatchItemOwnership(existing, &row); err != nil {
		return err
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
	if err := resolveStudioGenerationAttemptOwnership(existing, &row); err != nil {
		return err
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
	if err := resolveStudioMaterializedDesignOwnership(existing, &row); err != nil {
		return err
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
	attempts := make([]StudioGenerationAttemptRecord, 0)
	for _, attempt := range r.attempts {
		if !matchesStudioBatchScope(ctx, attempt.TenantID, attempt.UserID) {
			continue
		}
		attempts = append(attempts, attempt)
	}

	designs := make([]StudioMaterializedDesignRecord, 0)
	for _, design := range r.designs {
		if !matchesStudioBatchScope(ctx, design.TenantID, design.UserID) {
			continue
		}
		designs = append(designs, design)
	}

	return buildStudioBatchDetailGraph(&batch, items, attempts, designs)
}
