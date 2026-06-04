package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

type MemSheinSyncRepository struct {
	mu              sync.RWMutex
	nextProductID   int64
	nextJobID       int64
	nextCandidateID int64
	nextRunID       int64
	products        map[string]listingkit.SheinSyncedProductRecord
	syncJobs        map[int64]listingkit.SheinSyncJobRecord
	candidates      map[string]listingkit.SheinActivityCandidateRecord
	enrollmentRuns  map[int64]listingkit.SheinActivityEnrollmentRunRecord
	enrollmentItems map[string]listingkit.SheinActivityEnrollmentItemRecord
}

func NewMemSheinSyncRepository() listingkit.SheinSyncRepository {
	return &MemSheinSyncRepository{
		nextProductID:   1,
		nextJobID:       1,
		nextCandidateID: 1,
		nextRunID:       1,
		products:        make(map[string]listingkit.SheinSyncedProductRecord),
		syncJobs:        make(map[int64]listingkit.SheinSyncJobRecord),
		candidates:      make(map[string]listingkit.SheinActivityCandidateRecord),
		enrollmentRuns:  make(map[int64]listingkit.SheinActivityEnrollmentRunRecord),
		enrollmentItems: make(map[string]listingkit.SheinActivityEnrollmentItemRecord),
	}
}

func (r *MemSheinSyncRepository) UpsertSyncedProducts(_ context.Context, records []*listingkit.SheinSyncedProductRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for _, record := range records {
		if record == nil {
			continue
		}

		key := sheinSyncedProductKey(record.TenantID, record.StoreID, record.SKCName)
		row := cloneSheinSyncedProductRecord(*record)
		listingkit.ApplyEffectiveCostPrice(&row)
		existing, ok := r.products[key]
		if ok {
			row.ID = existing.ID
			if row.CreatedAt.IsZero() {
				row.CreatedAt = existing.CreatedAt
			}
		} else {
			row.ID = r.nextProductID
			r.nextProductID++
			if row.CreatedAt.IsZero() {
				row.CreatedAt = now
			}
		}
		if row.UpdatedAt.IsZero() {
			row.UpdatedAt = now
		}
		if row.LastSyncAt == nil {
			row.LastSyncAt = cloneTimePtr(&now)
		}
		r.products[key] = row
	}
	return nil
}

func (r *MemSheinSyncRepository) ListSyncedProducts(_ context.Context, query *listingkit.SheinSyncedProductQuery) ([]listingkit.SheinSyncedProductRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	items := make([]listingkit.SheinSyncedProductRecord, 0, len(r.products))
	for _, row := range r.products {
		if !matchesSheinSyncedProductQuery(row, query) {
			continue
		}
		items = append(items, cloneSheinSyncedProductRecord(row))
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].ID > items[j].ID
	})

	total := int64(len(items))
	page, pageSize := sheinSyncQueryPage(query)
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []listingkit.SheinSyncedProductRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func (r *MemSheinSyncRepository) UpdateManualCostPrice(_ context.Context, productID int64, manualCostPrice *float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, row := range r.products {
		if row.ID != productID {
			continue
		}
		row.ManualCostPrice = cloneFloat64Ptr(manualCostPrice)
		listingkit.ApplyEffectiveCostPrice(&row)
		row.UpdatedAt = time.Now().UTC()
		r.products[key] = row
		return nil
	}
	return gorm.ErrRecordNotFound
}

func (r *MemSheinSyncRepository) MarkMissingSyncedProductsInactive(_ context.Context, tenantID, storeID int64, activeSKCNames []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	activeSet := make(map[string]struct{}, len(activeSKCNames))
	for _, skcName := range activeSKCNames {
		activeSet[skcName] = struct{}{}
	}

	for key, row := range r.products {
		if row.TenantID != tenantID || row.StoreID != storeID {
			continue
		}
		if _, ok := activeSet[row.SKCName]; ok {
			continue
		}
		row.IsActive = false
		row.UpdatedAt = time.Now().UTC()
		r.products[key] = row
	}
	return nil
}

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
	return items[start:end], total, nil
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

func matchesSheinSyncedProductQuery(row listingkit.SheinSyncedProductRecord, query *listingkit.SheinSyncedProductQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.SKCName != "" && row.SKCName != query.SKCName {
		return false
	}
	if query.IsActive != nil && row.IsActive != *query.IsActive {
		return false
	}
	return true
}

func matchesSheinSyncJobQuery(row listingkit.SheinSyncJobRecord, query *listingkit.SheinSyncJobQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.TriggerMode != nil && row.TriggerMode != *query.TriggerMode {
		return false
	}
	if query.Status != nil && row.Status != *query.Status {
		return false
	}
	if query.StartedFrom != nil && (row.StartedAt == nil || row.StartedAt.Before(*query.StartedFrom)) {
		return false
	}
	if query.StartedTo != nil && (row.StartedAt == nil || row.StartedAt.After(*query.StartedTo)) {
		return false
	}
	return true
}

func matchesSheinActivityCandidateQuery(row listingkit.SheinActivityCandidateRecord, query *listingkit.SheinActivityCandidateQuery) bool {
	if query == nil {
		return true
	}
	if query.TenantID > 0 && row.TenantID != query.TenantID {
		return false
	}
	if query.StoreID > 0 && row.StoreID != query.StoreID {
		return false
	}
	if query.ActivityType != "" && row.ActivityType != query.ActivityType {
		return false
	}
	if query.ActivityKey != "" && row.ActivityKey != query.ActivityKey {
		return false
	}
	if query.SKCName != "" && row.SKCName != query.SKCName {
		return false
	}
	if query.CandidateVersion != "" && row.CandidateVersion != query.CandidateVersion {
		return false
	}
	if len(query.CandidateIDs) > 0 {
		found := false
		for _, id := range query.CandidateIDs {
			if row.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func cloneSheinSyncedProductRecord(row listingkit.SheinSyncedProductRecord) listingkit.SheinSyncedProductRecord {
	row.PublishTime = cloneTimePtr(row.PublishTime)
	row.FirstShelfTime = cloneTimePtr(row.FirstShelfTime)
	row.LastSyncAt = cloneTimePtr(row.LastSyncAt)
	row.AutoCostPrice = cloneFloat64Ptr(row.AutoCostPrice)
	row.ManualCostPrice = cloneFloat64Ptr(row.ManualCostPrice)
	row.EffectiveCostPrice = cloneFloat64Ptr(row.EffectiveCostPrice)
	return row
}

func cloneSheinSyncJobRecord(row listingkit.SheinSyncJobRecord) listingkit.SheinSyncJobRecord {
	row.StartedAt = cloneTimePtr(row.StartedAt)
	row.FinishedAt = cloneTimePtr(row.FinishedAt)
	return row
}

func cloneSheinCandidateRecord(row listingkit.SheinActivityCandidateRecord) listingkit.SheinActivityCandidateRecord {
	row.EffectiveCostPrice = cloneFloat64Ptr(row.EffectiveCostPrice)
	row.CalculatedProfitRate = cloneFloat64Ptr(row.CalculatedProfitRate)
	return row
}

func cloneSheinEnrollmentRunRecord(row listingkit.SheinActivityEnrollmentRunRecord) listingkit.SheinActivityEnrollmentRunRecord {
	row.StartedAt = cloneTimePtr(row.StartedAt)
	row.FinishedAt = cloneTimePtr(row.FinishedAt)
	return row
}
