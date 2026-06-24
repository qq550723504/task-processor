package store

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/listingkit"
)

func (r *MemSheinSyncRepository) UpsertSDSCostGroup(_ context.Context, record *listingkit.SheinSDSCostGroupRecord) error {
	if record == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	row := *record
	row.GroupKey = strings.TrimSpace(row.GroupKey)
	row.GroupLabel = strings.TrimSpace(row.GroupLabel)
	key := sheinSDSCostGroupKey(row.TenantID, row.StoreID, row.GroupKey)
	if existing, ok := r.sdsCostGroups[key]; ok {
		row.ID = existing.ID
		row.CreatedAt = existing.CreatedAt
		if row.GroupLabel == "" {
			row.GroupLabel = existing.GroupLabel
		}
	} else {
		row.ID = int64(len(r.sdsCostGroups) + 1)
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
	}
	row.UpdatedAt = now
	r.sdsCostGroups[key] = row
	return nil
}

func (r *MemSheinSyncRepository) ListSDSCostGroups(_ context.Context, query *listingkit.SheinSDSCostGroupQuery) ([]listingkit.SheinSDSCostGroupRecord, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	groupKeys := map[string]struct{}{}
	if query != nil {
		for _, key := range normalizedSheinSDSGroupKeys(query.GroupKeys) {
			groupKeys[key] = struct{}{}
		}
	}
	items := make([]listingkit.SheinSDSCostGroupRecord, 0, len(r.sdsCostGroups))
	for _, row := range r.sdsCostGroups {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
			if len(groupKeys) > 0 {
				if _, ok := groupKeys[row.GroupKey]; !ok {
					continue
				}
			}
		}
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].GroupKey != items[j].GroupKey {
			return items[i].GroupKey < items[j].GroupKey
		}
		return items[i].ID < items[j].ID
	})

	total := int64(len(items))
	page, pageSize := 1, len(items)
	if query != nil {
		page, pageSize = normalizeSheinSyncPage(query.Page, query.PageSize)
	}
	if pageSize == 0 {
		pageSize = 20
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []listingkit.SheinSDSCostGroupRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total, nil
}

func sheinSDSCostGroupKey(tenantID, storeID int64, groupKey string) string {
	return strconv.FormatInt(tenantID, 10) + "|" + strconv.FormatInt(storeID, 10) + "|" + strings.TrimSpace(groupKey)
}

func normalizedSheinSDSGroupKeys(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}
