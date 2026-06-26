package store

import (
	"context"
	"sort"

	"task-processor/internal/listingkit"
)

func (r *GormSheinSyncRepository) ListSourceSDSCostGroups(ctx context.Context, query *listingkit.SheinSourceSDSCostGroupQuery) ([]listingkit.SheinSourceSDSCostGroupRecord, int64, error) {
	db := r.db.WithContext(ctx).Model(&listingkit.SheinSyncedProductRecord{})
	if query != nil {
		if query.TenantID > 0 {
			db = db.Where("tenant_id = ?", query.TenantID)
		}
		if query.StoreID > 0 {
			db = db.Where("store_id = ?", query.StoreID)
		}
	}

	var products []listingkit.SheinSyncedProductRecord
	if err := db.Order("supplier_code ASC, skc_name ASC, id ASC").Find(&products).Error; err != nil {
		return nil, 0, err
	}

	groups, allKeys := buildSheinSourceSDSCostGroupRows(products)
	if err := r.applySheinSDSCostGroupCosts(ctx, groups, allKeys, query); err != nil {
		return nil, 0, err
	}
	return paginateSheinSourceSDSCostGroupRows(groups, query), int64(len(groups)), nil
}

func (r *MemSheinSyncRepository) ListSourceSDSCostGroups(ctx context.Context, query *listingkit.SheinSourceSDSCostGroupQuery) ([]listingkit.SheinSourceSDSCostGroupRecord, int64, error) {
	r.mu.RLock()
	products := make([]listingkit.SheinSyncedProductRecord, 0, len(r.products))
	for _, row := range r.products {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
		}
		products = append(products, cloneSheinSyncedProductRecord(row))
	}
	sort.Slice(products, func(i, j int) bool {
		if products[i].SupplierCode != products[j].SupplierCode {
			return products[i].SupplierCode < products[j].SupplierCode
		}
		if products[i].SKCName != products[j].SKCName {
			return products[i].SKCName < products[j].SKCName
		}
		return products[i].ID < products[j].ID
	})

	costs := make(map[string]*float64)
	for _, row := range r.sdsCostGroups {
		if query != nil {
			if query.TenantID > 0 && row.TenantID != query.TenantID {
				continue
			}
			if query.StoreID > 0 && row.StoreID != query.StoreID {
				continue
			}
		}
		costs[row.GroupKey] = cloneFloat64Ptr(row.ManualCostPrice)
	}
	r.mu.RUnlock()

	groups, _ := buildSheinSourceSDSCostGroupRows(products)
	applySheinSourceSDSManualCosts(groups, costs)
	return paginateSheinSourceSDSCostGroupRows(groups, query), int64(len(groups)), nil
}

func (r *GormSheinSyncRepository) applySheinSDSCostGroupCosts(
	ctx context.Context,
	groups []listingkit.SheinSourceSDSCostGroupRecord,
	allKeys []string,
	query *listingkit.SheinSourceSDSCostGroupQuery,
) error {
	if len(groups) == 0 || len(allKeys) == 0 {
		return nil
	}

	db := r.db.WithContext(ctx).Model(&listingkit.SheinSDSCostGroupRecord{}).Where("group_key IN ?", normalizedSheinSDSGroupKeys(allKeys))
	if query != nil {
		if query.TenantID > 0 {
			db = db.Where("tenant_id = ?", query.TenantID)
		}
		if query.StoreID > 0 {
			db = db.Where("store_id = ?", query.StoreID)
		}
	}
	var rows []listingkit.SheinSDSCostGroupRecord
	if err := db.Find(&rows).Error; err != nil {
		return err
	}

	costs := make(map[string]*float64, len(rows))
	for _, row := range rows {
		costs[row.GroupKey] = cloneFloat64Ptr(row.ManualCostPrice)
	}
	applySheinSourceSDSManualCosts(groups, costs)
	return nil
}

func buildSheinSourceSDSCostGroupRows(products []listingkit.SheinSyncedProductRecord) ([]listingkit.SheinSourceSDSCostGroupRecord, []string) {
	rowsByKey := make(map[string]*listingkit.SheinSourceSDSCostGroupRecord)
	keySeen := make(map[string]struct{})
	allKeys := make([]string, 0)

	for _, product := range products {
		identity := listingkit.ResolveSheinSDSCostGroupIdentity(product)
		if identity.GroupKey == "" || identity.SourceCode == "" {
			continue
		}
		row := rowsByKey[identity.GroupKey]
		if row == nil {
			rowsByKey[identity.GroupKey] = &listingkit.SheinSourceSDSCostGroupRecord{
				GroupKey:        identity.GroupKey,
				GroupLabel:      identity.GroupLabel,
				SourceCode:      identity.SourceCode,
				LegacyGroupKeys: append([]string(nil), identity.LegacyGroupKeys...),
			}
			row = rowsByKey[identity.GroupKey]
		} else {
			row.LegacyGroupKeys = appendMissingSheinSourceSDSGroupKeys(row.LegacyGroupKeys, identity.LegacyGroupKeys)
		}
		row.ProductCount++
		if len(row.Products) < 5 {
			row.Products = append(row.Products, product)
		}
		for _, key := range append([]string{identity.GroupKey}, identity.LegacyGroupKeys...) {
			if key == "" {
				continue
			}
			if _, ok := keySeen[key]; ok {
				continue
			}
			keySeen[key] = struct{}{}
			allKeys = append(allKeys, key)
		}
	}

	rows := make([]listingkit.SheinSourceSDSCostGroupRecord, 0, len(rowsByKey))
	for _, row := range rowsByKey {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].SourceCode != rows[j].SourceCode {
			return rows[i].SourceCode < rows[j].SourceCode
		}
		return rows[i].GroupKey < rows[j].GroupKey
	})
	return rows, allKeys
}

func appendMissingSheinSourceSDSGroupKeys(existing []string, next []string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, key := range existing {
		seen[key] = struct{}{}
	}
	for _, key := range next {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		existing = append(existing, key)
	}
	return existing
}

func applySheinSourceSDSManualCosts(rows []listingkit.SheinSourceSDSCostGroupRecord, costs map[string]*float64) {
	for i := range rows {
		if cost, ok := costs[rows[i].GroupKey]; ok {
			rows[i].ManualCostPrice = cloneFloat64Ptr(cost)
			continue
		}
		for _, legacyKey := range rows[i].LegacyGroupKeys {
			if cost, ok := costs[legacyKey]; ok {
				rows[i].ManualCostPrice = cloneFloat64Ptr(cost)
				break
			}
		}
	}
}

func paginateSheinSourceSDSCostGroupRows(
	rows []listingkit.SheinSourceSDSCostGroupRecord,
	query *listingkit.SheinSourceSDSCostGroupQuery,
) []listingkit.SheinSourceSDSCostGroupRecord {
	page, pageSize := normalizeSheinSyncPage(0, 0)
	if query != nil {
		page, pageSize = normalizeSheinSyncPage(query.Page, query.PageSize)
	}
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []listingkit.SheinSourceSDSCostGroupRecord{}
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end]
}
