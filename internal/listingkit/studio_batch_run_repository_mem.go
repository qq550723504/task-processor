package listingkit

import (
	"context"
	"slices"
	"sync"

	"gorm.io/gorm"
)

type MemStudioBatchRunRepository struct {
	mu    sync.Mutex
	runs  map[string]StudioBatchRunRecord
	items map[string]StudioBatchRunItemRecord
}

func NewMemStudioBatchRunRepository() *MemStudioBatchRunRepository {
	return &MemStudioBatchRunRepository{
		runs:  map[string]StudioBatchRunRecord{},
		items: map[string]StudioBatchRunItemRecord{},
	}
}

func (r *MemStudioBatchRunRepository) CreateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord, items []StudioBatchRunItemRecord) error {
	if run == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	runRow := *run
	applyStudioBatchRunScopeDefaults(ctx, &runRow.TenantID, &runRow.UserID)
	r.runs[runRow.ID] = runRow

	for _, item := range items {
		itemRow := item
		itemRow.TenantID = runRow.TenantID
		itemRow.UserID = runRow.UserID
		if itemRow.RunID == "" {
			itemRow.RunID = runRow.ID
		}
		r.items[itemRow.ID] = itemRow
	}
	return nil
}

func (r *MemStudioBatchRunRepository) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.runs[runID]
	if !ok || !matchesStudioBatchRunScope(ctx, record.TenantID, record.UserID) {
		return nil, gorm.ErrRecordNotFound
	}
	cloned := record
	return &cloned, nil
}

func (r *MemStudioBatchRunRepository) ListUnfinishedStudioBatchRuns(ctx context.Context) ([]StudioBatchRunRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	runs := make([]StudioBatchRunRecord, 0)
	for _, run := range r.runs {
		if !matchesStudioBatchRunScope(ctx, run.TenantID, run.UserID) {
			continue
		}
		if run.Status != StudioBatchRunStatusPending && run.Status != StudioBatchRunStatusRunning {
			continue
		}
		runs = append(runs, run)
	}
	slices.SortStableFunc(runs, func(a, b StudioBatchRunRecord) int {
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
	return runs, nil
}

func (r *MemStudioBatchRunRepository) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	run, ok := r.runs[runID]
	if !ok || !matchesStudioBatchRunScope(ctx, run.TenantID, run.UserID) {
		return nil, gorm.ErrRecordNotFound
	}

	items := make([]StudioBatchRunItemRecord, 0)
	for _, item := range r.items {
		if item.RunID != runID || !matchesStudioBatchRunScope(ctx, item.TenantID, item.UserID) {
			continue
		}
		items = append(items, item)
	}
	slices.SortStableFunc(items, func(a, b StudioBatchRunItemRecord) int {
		if a.Position < b.Position {
			return -1
		}
		if a.Position > b.Position {
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
	return items, nil
}

func (r *MemStudioBatchRunRepository) ListStudioBatchRunItemsByBatchID(ctx context.Context, batchID string) ([]StudioBatchRunItemRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]StudioBatchRunItemRecord, 0)
	for _, item := range r.items {
		if item.BatchID != batchID || !matchesStudioBatchRunScope(ctx, item.TenantID, item.UserID) {
			continue
		}
		items = append(items, item)
	}
	slices.SortStableFunc(items, func(a, b StudioBatchRunItemRecord) int {
		if a.UpdatedAt.After(b.UpdatedAt) {
			return -1
		}
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return 1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.Before(b.CreatedAt) {
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
	return items, nil
}

func (r *MemStudioBatchRunRepository) UpdateStudioBatchRun(ctx context.Context, run *StudioBatchRunRecord) error {
	if run == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.runs[run.ID]
	if !ok || !matchesStudioBatchRunScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *run
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	}
	r.runs[row.ID] = row
	return nil
}

func (r *MemStudioBatchRunRepository) UpdateStudioBatchRunItem(ctx context.Context, item *StudioBatchRunItemRecord) error {
	if item == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.items[item.ID]
	if !ok || !matchesStudioBatchRunScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}
	row := *item
	if row.TenantID == "" {
		row.TenantID = existing.TenantID
	}
	if row.UserID == "" {
		row.UserID = existing.UserID
	}
	if row.RunID == "" {
		row.RunID = existing.RunID
	}
	r.items[row.ID] = row
	return nil
}
