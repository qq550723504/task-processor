package store

import (
	"context"
	"sort"

	"task-processor/internal/listingkit"
)

func (r *GormSheinSyncRepository) ListSourceSDSCostGroups(ctx context.Context, query *listingkit.SheinSourceSDSCostGroupQuery) ([]listingkit.SheinSourceSDSCostGroupRecord, int64, error) {
	db := r.db.WithContext(ctx).
		Model(&listingkit.SheinSyncedProductRecord{}).
		Where("is_active = ?", true).
		Where("shelf_status = '' OR shelf_status IS NULL OR shelf_status = ?", "ON_SHELF")
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
		if !sheinSyncedProductVisibleForSDSCostMaintenance(row) {
			continue
		}
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

func sheinSyncedProductVisibleForSDSCostMaintenance(row listingkit.SheinSyncedProductRecord) bool {
	if !row.IsActive {
		return false
	}
	return row.ShelfStatus == "" || row.ShelfStatus == "ON_SHELF"
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
		row.SKUCodes = appendMissingSheinSourceSDSGroupKeys(row.SKUCodes, listingkit.SheinSyncedProductSKUCodes(product))
		variantIdentities := listingkit.ResolveSheinSDSVariantCostGroupIdentities(product)
		for _, variantIdentity := range variantIdentities {
			row.SKUGroups = appendSheinSourceSDSVariantGroupProduct(row.SKUGroups, variantIdentity, product)
		}
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
		for _, variantIdentity := range variantIdentities {
			for _, key := range append([]string{variantIdentity.GroupKey}, variantIdentity.LegacyGroupKeys...) {
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
	}

	rows := make([]listingkit.SheinSourceSDSCostGroupRecord, 0, len(rowsByKey))
	for _, row := range rowsByKey {
		sort.Strings(row.SKUCodes)
		for i := range row.SKUGroups {
			sort.Strings(row.SKUGroups[i].SKUCodes)
		}
		sort.Slice(row.SKUGroups, func(i, j int) bool {
			if row.SKUGroups[i].VariantLabel != row.SKUGroups[j].VariantLabel {
				return row.SKUGroups[i].VariantLabel < row.SKUGroups[j].VariantLabel
			}
			return row.SKUGroups[i].GroupKey < row.SKUGroups[j].GroupKey
		})
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

func appendSheinSourceSDSVariantGroupProduct(
	existing []listingkit.SheinSourceSDSSKUCostGroupRecord,
	identity listingkit.SheinSDSCostGroupIdentity,
	product listingkit.SheinSyncedProductRecord,
) []listingkit.SheinSourceSDSSKUCostGroupRecord {
	if identity.GroupKey == "" || identity.VariantLabel == "" {
		return existing
	}
	for i := range existing {
		if existing[i].GroupKey == identity.GroupKey {
			existing[i].ProductCount++
			existing[i].SKUCodes = appendMissingSheinSourceSDSGroupKeys(existing[i].SKUCodes, sheinSourceSDSVariantSKUCodes(identity, product))
			if len(existing[i].Products) < 5 {
				existing[i].Products = append(existing[i].Products, product)
			}
			return existing
		}
	}
	return append(existing, listingkit.SheinSourceSDSSKUCostGroupRecord{
		GroupKey:        identity.GroupKey,
		GroupLabel:      identity.GroupLabel,
		SourceCode:      identity.SourceCode,
		SKUCode:         identity.SKUCode,
		VariantLabel:    identity.VariantLabel,
		SKUCodes:        sheinSourceSDSVariantSKUCodes(identity, product),
		ProductCount:    1,
		Products:        []listingkit.SheinSyncedProductRecord{product},
		LegacyGroupKeys: append([]string(nil), identity.LegacyGroupKeys...),
	})
}

func sheinSourceSDSVariantSKUCodes(identity listingkit.SheinSDSCostGroupIdentity, product listingkit.SheinSyncedProductRecord) []string {
	if len(identity.SKUCodes) > 0 {
		return append([]string(nil), identity.SKUCodes...)
	}
	return listingkit.SheinSyncedProductSKUCodes(product)
}

func applySheinSourceSDSManualCosts(rows []listingkit.SheinSourceSDSCostGroupRecord, costs map[string]*float64) {
	for i := range rows {
		if cost, ok := costs[rows[i].GroupKey]; ok {
			rows[i].ManualCostPrice = cloneFloat64Ptr(cost)
		} else {
			for _, legacyKey := range rows[i].LegacyGroupKeys {
				if cost, ok := costs[legacyKey]; ok {
					rows[i].ManualCostPrice = cloneFloat64Ptr(cost)
					break
				}
			}
		}
		for j := range rows[i].SKUGroups {
			if cost, ok := costs[rows[i].SKUGroups[j].GroupKey]; ok {
				rows[i].SKUGroups[j].ManualCostPrice = cloneFloat64Ptr(cost)
				continue
			}
			for _, legacyKey := range rows[i].SKUGroups[j].LegacyGroupKeys {
				if cost, ok := costs[legacyKey]; ok {
					rows[i].SKUGroups[j].ManualCostPrice = cloneFloat64Ptr(cost)
					break
				}
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
