package listingadmin

import "testing"

func TestApplyFilterRuleDefaultsSetsFallbacks(t *testing.T) {
	t.Parallel()

	row := listingFilterRule{}
	applyFilterRuleDefaults(&row)

	if row.PriceMax != 99999 {
		t.Fatalf("priceMax = %v, want 99999", row.PriceMax)
	}
	if row.StockMin != 10 {
		t.Fatalf("stockMin = %d, want 10", row.StockMin)
	}
	if row.FulfillmentType != "ALL" {
		t.Fatalf("fulfillmentType = %q, want ALL", row.FulfillmentType)
	}
}

func TestApplyFilterRuleAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingFilterRule{}
	applyFilterRuleAuditFields(&row, "user-1", true)

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

func TestListingFilterRuleConversionPreservesOptionalFields(t *testing.T) {
	t.Parallel()

	storeID := int64(11)
	categoryID := int64(22)
	deliveryTimeMax := 7
	rule := FilterRule{
		ID:              1,
		TenantID:        101,
		Name:            " Basic ",
		RuleCode:        " FR-1 ",
		Description:     " desc ",
		StoreID:         &storeID,
		CategoryID:      &categoryID,
		PriceType:       " special ",
		PriceMin:        1.2,
		PriceMax:        30,
		StockMin:        9,
		RatingMin:       4.1,
		ReviewCountMin:  10,
		DeliveryTimeMax: &deliveryTimeMax,
		FulfillmentType: " FBA ",
		Status:          2,
		Remark:          " note ",
	}

	row := listingFilterRuleFromFilterRule(&rule)
	if row.Name != "Basic" || row.RuleCode != "FR-1" || row.PriceType != "special" {
		t.Fatalf("trimmed row = %+v, want trimmed strings", row)
	}
	if row.StoreID != storeID || row.CategoryID != categoryID || row.DeliveryTimeMax != deliveryTimeMax {
		t.Fatalf("row optionals = %+v, want preserved numeric values", row)
	}

	converted := row.toFilterRule()
	if converted.StoreID == nil || *converted.StoreID != storeID {
		t.Fatalf("converted storeID = %v, want %d", converted.StoreID, storeID)
	}
	if converted.CategoryID == nil || *converted.CategoryID != categoryID {
		t.Fatalf("converted categoryID = %v, want %d", converted.CategoryID, categoryID)
	}
	if converted.DeliveryTimeMax == nil || *converted.DeliveryTimeMax != deliveryTimeMax {
		t.Fatalf("converted deliveryTimeMax = %v, want %d", converted.DeliveryTimeMax, deliveryTimeMax)
	}
	if converted.Description != "desc" || converted.Remark != "note" {
		t.Fatalf("converted = %+v, want trimmed values preserved", converted)
	}
}
