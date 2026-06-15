package store

import (
	"context"
	"sort"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

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
