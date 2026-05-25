package listingadmin

import "testing"

func TestApplyProductImportMappingDefaultsSetsMultipliers(t *testing.T) {
	t.Parallel()

	row := listingProductImportMapping{}
	applyProductImportMappingDefaults(&row)

	if row.SalePriceMultiplier != 1 {
		t.Fatalf("salePriceMultiplier = %v, want 1", row.SalePriceMultiplier)
	}
	if row.DiscountPriceMultiplier != 1 {
		t.Fatalf("discountPriceMultiplier = %v, want 1", row.DiscountPriceMultiplier)
	}
}

func TestApplyProductImportMappingAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingProductImportMapping{}
	applyProductImportMappingAuditFields(&row, "user-1", true)

	if row.OwnerUserID != "user-1" {
		t.Fatalf("ownerUserID = %q, want user-1", row.OwnerUserID)
	}
	if row.Creator != "user-1" || row.CreatedBy != "user-1" {
		t.Fatalf("creator/createdBy = %q/%q, want user-1/user-1", row.Creator, row.CreatedBy)
	}
	if row.Updater != "user-1" || row.UpdatedBy != "user-1" {
		t.Fatalf("updater/updatedBy = %q/%q, want user-1/user-1", row.Updater, row.UpdatedBy)
	}
}

func TestListingProductImportMappingConversionPreservesOptionalFields(t *testing.T) {
	t.Parallel()

	costPrice := 19.9
	filterRuleID := int64(12)
	profitRuleID := int64(34)
	mapping := ProductImportMapping{
		ID:                      1,
		TenantID:                101,
		ImportTaskID:            201,
		StoreID:                 301,
		Platform:                " SHEIN ",
		Region:                  " US ",
		ProductID:               " P-1 ",
		ParentProductID:         " Parent-1 ",
		SKU:                     " SKU-1 ",
		CostPrice:               &costPrice,
		PlatformProductID:       " PP-1 ",
		PlatformParentProductID: " PPP-1 ",
		FilterRuleID:            &filterRuleID,
		FilterRuleRange:         " all ",
		ProfitRuleID:            &profitRuleID,
		SalePriceMultiplier:     1.5,
		DiscountPriceMultiplier: 0.8,
		Status:                  2,
		Remark:                  " ok ",
	}

	row := listingProductImportMappingFromProductImportMapping(&mapping)
	if row.Platform != "SHEIN" || row.Region != "US" || row.ProductID != "P-1" {
		t.Fatalf("trimmed row = %+v, want trimmed platform/region/productID", row)
	}
	if row.CostPrice != costPrice || row.FilterRuleID != filterRuleID || row.ProfitRuleID != profitRuleID {
		t.Fatalf("row optional values = %+v, want preserved numeric optionals", row)
	}

	converted := row.toProductImportMapping()
	if converted.CostPrice == nil || *converted.CostPrice != costPrice {
		t.Fatalf("converted costPrice = %v, want %v", converted.CostPrice, costPrice)
	}
	if converted.FilterRuleID == nil || *converted.FilterRuleID != filterRuleID {
		t.Fatalf("converted filterRuleID = %v, want %d", converted.FilterRuleID, filterRuleID)
	}
	if converted.ProfitRuleID == nil || *converted.ProfitRuleID != profitRuleID {
		t.Fatalf("converted profitRuleID = %v, want %d", converted.ProfitRuleID, profitRuleID)
	}
	if converted.Remark != "ok" || converted.SKU != "SKU-1" {
		t.Fatalf("converted = %+v, want trimmed remark and sku", converted)
	}
}
