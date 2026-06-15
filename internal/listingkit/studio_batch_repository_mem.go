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
