package store

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *MemSheinSyncRepository) SaveSyncJob(_ context.Context, job *listingkit.SheinSyncJobRecord) error {
	if job == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	row := cloneSheinSyncJobRecord(*job)
	now := time.Now().UTC()
	if row.ID <= 0 {
		row.ID = r.nextJobID
		r.nextJobID++
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
	} else {
		existing, ok := r.syncJobs[row.ID]
		if !ok {
			return gorm.ErrRecordNotFound
		}
		if row.CreatedAt.IsZero() {
			row.CreatedAt = existing.CreatedAt
		}
	}
	row.UpdatedAt = now
	r.syncJobs[row.ID] = row
	*job = cloneSheinSyncJobRecord(row)
	return nil
}

func (r *MemSheinSyncRepository) ListSyncJobs(_ context.Context, query *listingkit.SheinSyncJobQuery) ([]listingkit.SheinSyncJobRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]listingkit.SheinSyncJobRecord, 0, len(r.syncJobs))
	for _, row := range r.syncJobs {
		if !matchesSheinSyncJobQuery(row, query) {
			continue
		}
		items = append(items, cloneSheinSyncJobRecord(row))
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i].StartedAt
		right := items[j].StartedAt
		switch {
		case left == nil && right != nil:
			return false
		case left != nil && right == nil:
			return true
		case left != nil && right != nil && !left.Equal(*right):
			return left.After(*right)
		default:
			return items[i].ID > items[j].ID
		}
	})

	total := int64(len(items))
	page, pageSize := sheinSyncJobQueryPage(query)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []listingkit.SheinSyncJobRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}
