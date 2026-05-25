package listingadmin

import "strings"

func (r listingProductImportMapping) toProductImportMapping() ProductImportMapping {
	return ProductImportMapping{
		ID:                      r.ID,
		TenantID:                r.TenantID,
		ImportTaskID:            r.ImportTaskID,
		StoreID:                 r.StoreID,
		Platform:                r.Platform,
		Region:                  r.Region,
		ProductID:               r.ProductID,
		ParentProductID:         r.ParentProductID,
		SKU:                     r.SKU,
		CostPrice:               floatPtrIfPositive(r.CostPrice),
		PlatformProductID:       r.PlatformProductID,
		PlatformParentProductID: r.PlatformParentProductID,
		FilterRuleID:            int64PtrIfPositive(r.FilterRuleID),
		FilterRuleRange:         r.FilterRuleRange,
		ProfitRuleID:            int64PtrIfPositive(r.ProfitRuleID),
		SalePriceMultiplier:     r.SalePriceMultiplier,
		DiscountPriceMultiplier: r.DiscountPriceMultiplier,
		Status:                  r.Status,
		Remark:                  r.Remark,
		CreateTime:              r.CreateTime,
		UpdateTime:              r.UpdateTime,
	}
}

func listingProductImportMappingFromProductImportMapping(mapping *ProductImportMapping) listingProductImportMapping {
	if mapping == nil {
		return listingProductImportMapping{}
	}
	return listingProductImportMapping{
		ID:                      mapping.ID,
		TenantID:                mapping.TenantID,
		ImportTaskID:            mapping.ImportTaskID,
		StoreID:                 mapping.StoreID,
		Platform:                strings.TrimSpace(mapping.Platform),
		Region:                  strings.TrimSpace(mapping.Region),
		ProductID:               strings.TrimSpace(mapping.ProductID),
		ParentProductID:         strings.TrimSpace(mapping.ParentProductID),
		SKU:                     strings.TrimSpace(mapping.SKU),
		CostPrice:               floatValue(mapping.CostPrice),
		PlatformProductID:       strings.TrimSpace(mapping.PlatformProductID),
		PlatformParentProductID: strings.TrimSpace(mapping.PlatformParentProductID),
		FilterRuleID:            int64Value(mapping.FilterRuleID),
		FilterRuleRange:         strings.TrimSpace(mapping.FilterRuleRange),
		ProfitRuleID:            int64Value(mapping.ProfitRuleID),
		SalePriceMultiplier:     mapping.SalePriceMultiplier,
		DiscountPriceMultiplier: mapping.DiscountPriceMultiplier,
		Status:                  mapping.Status,
		Remark:                  strings.TrimSpace(mapping.Remark),
	}
}

func applyProductImportMappingDefaults(row *listingProductImportMapping) {
	if row.SalePriceMultiplier == 0 {
		row.SalePriceMultiplier = 1
	}
	if row.DiscountPriceMultiplier == 0 {
		row.DiscountPriceMultiplier = 1
	}
}

func applyProductImportMappingAuditFields(row *listingProductImportMapping, userID string, includeCreate bool) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}
	row.OwnerUserID = trimmedUserID
	row.Updater = trimmedUserID
	row.UpdatedBy = trimmedUserID
	if includeCreate {
		row.Creator = trimmedUserID
		row.CreatedBy = trimmedUserID
	}
}
