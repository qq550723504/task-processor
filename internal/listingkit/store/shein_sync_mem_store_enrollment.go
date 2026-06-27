package store

import (
	"context"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *MemSheinSyncRepository) SaveCandidates(_ context.Context, records []*listingkit.SheinActivityCandidateRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for _, record := range records {
		if record == nil {
			continue
		}

		row := cloneSheinCandidateRecord(*record)
		key := sheinCandidateKey(row)
		existing, ok := r.candidates[key]
		if ok {
			row.ID = existing.ID
			if row.CreatedAt.IsZero() {
				row.CreatedAt = existing.CreatedAt
			}
		} else {
			row.ID = r.nextCandidateID
			r.nextCandidateID++
			if row.CreatedAt.IsZero() {
				row.CreatedAt = now
			}
		}
		row.UpdatedAt = now
		r.candidates[key] = row
	}
	return nil
}

func (r *MemSheinSyncRepository) ListCandidates(_ context.Context, query *listingkit.SheinActivityCandidateQuery) ([]listingkit.SheinActivityCandidateRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]listingkit.SheinActivityCandidateRecord, 0, len(r.candidates))
	for _, row := range r.candidates {
		if !matchesSheinActivityCandidateQuery(row, query) {
			continue
		}
		items = append(items, cloneSheinCandidateRecord(row))
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].ID > items[j].ID
	})

	total := int64(len(items))
	page, pageSize := sheinActivityCandidateQueryPage(query)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []listingkit.SheinActivityCandidateRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	pageItems := items[start:end]
	r.attachLatestFailedEnrollmentErrorsLocked(pageItems)
	return pageItems, total, nil
}

func (r *MemSheinSyncRepository) attachLatestFailedEnrollmentErrorsLocked(rows []listingkit.SheinActivityCandidateRecord) {
	for i := range rows {
		if rows[i].ID <= 0 || rows[i].ReviewStatus != listingkit.SheinCandidateReviewStatusFailed {
			continue
		}
		var latest *listingkit.SheinActivityEnrollmentItemRecord
		for _, item := range r.enrollmentItems {
			if item.CandidateID != rows[i].ID ||
				item.Status != listingkit.SheinEnrollmentItemStatusFailed ||
				item.ErrorMessage == "" {
				continue
			}
			item := item
			if latest == nil || isNewerSheinEnrollmentItem(item, *latest) {
				latest = &item
			}
		}
		if latest != nil {
			rows[i].LastEnrollmentError = latest.ErrorMessage
		}
	}
}

func isNewerSheinEnrollmentItem(left, right listingkit.SheinActivityEnrollmentItemRecord) bool {
	if !left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.UpdatedAt.After(right.UpdatedAt)
	}
	return left.ID > right.ID
}

func (r *MemSheinSyncRepository) CreateEnrollmentRun(_ context.Context, run *listingkit.SheinActivityEnrollmentRunRecord) error {
	if run == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	row := cloneSheinEnrollmentRunRecord(*run)
	now := time.Now().UTC()
	row.ID = r.nextRunID
	r.nextRunID++
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	r.enrollmentRuns[row.ID] = row
	*run = cloneSheinEnrollmentRunRecord(row)
	return nil
}

func (r *MemSheinSyncRepository) UpdateEnrollmentRun(_ context.Context, run *listingkit.SheinActivityEnrollmentRunRecord) error {
	if run == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.enrollmentRuns[run.ID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	row := cloneSheinEnrollmentRunRecord(*run)
	if row.CreatedAt.IsZero() {
		row.CreatedAt = existing.CreatedAt
	}
	row.UpdatedAt = time.Now().UTC()
	r.enrollmentRuns[row.ID] = row
	*run = cloneSheinEnrollmentRunRecord(row)
	return nil
}

func (r *MemSheinSyncRepository) ListEnrollmentRuns(_ context.Context, query *listingkit.SheinEnrollmentRunQuery) ([]listingkit.SheinActivityEnrollmentRunRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]listingkit.SheinActivityEnrollmentRunRecord, 0, len(r.enrollmentRuns))
	for _, row := range r.enrollmentRuns {
		if !matchesSheinEnrollmentRunQuery(row, query) {
			continue
		}
		items = append(items, cloneSheinEnrollmentRunRecord(row))
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
	page, pageSize := sheinEnrollmentRunQueryPage(query)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []listingkit.SheinActivityEnrollmentRunRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *MemSheinSyncRepository) SaveEnrollmentItems(_ context.Context, items []*listingkit.SheinActivityEnrollmentItemRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	var nextID int64 = int64(len(r.enrollmentItems) + 1)
	for _, item := range items {
		if item == nil {
			continue
		}
		row := *item
		key := fmt.Sprintf("%d:%d", row.RunID, row.CandidateID)
		if existing, ok := r.enrollmentItems[key]; ok {
			row.ID = existing.ID
			if row.CreatedAt.IsZero() {
				row.CreatedAt = existing.CreatedAt
			}
		} else {
			if row.ID <= 0 {
				row.ID = nextID
				nextID++
			}
			if row.CreatedAt.IsZero() {
				row.CreatedAt = now
			}
		}
		row.UpdatedAt = now
		r.enrollmentItems[key] = row
	}
	return nil
}
