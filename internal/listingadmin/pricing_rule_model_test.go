package listingadmin

import "testing"

func TestApplyPricingRuleDefaultsSetsPriceMax(t *testing.T) {
	t.Parallel()

	row := listingPricingRule{}
	applyPricingRuleDefaults(&row)

	if row.PriceMax != 99999 {
		t.Fatalf("priceMax = %v, want 99999", row.PriceMax)
	}
}

func TestApplyPricingRuleAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingPricingRule{}
	applyPricingRuleAuditFields(&row, "user-1", true)

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

func TestListingPricingRuleConversionPreservesOptionalFields(t *testing.T) {
	t.Parallel()

	storeID := int64(11)
	categoryID := int64(22)
	fixedValue := 3.5
	rule := PricingRule{
		ID:              1,
		TenantID:        101,
		Name:            " Standard ",
		RuleCode:        " P-1 ",
		Description:     " desc ",
		Remark:          " note ",
		StoreID:         &storeID,
		CategoryID:      &categoryID,
		PriceMin:        10,
		PriceMax:        30,
		RuleType:        " ratio ",
		RuleValue:       1.2,
		FixedValue:      &fixedValue,
		AcceptCondition: " ok ",
		RejectCondition: " no ",
		Status:          2,
	}

	row := listingPricingRuleFromPricingRule(&rule)
	if row.Name != "Standard" || row.RuleCode != "P-1" || row.RuleType != "ratio" {
		t.Fatalf("trimmed row = %+v, want trimmed strings", row)
	}
	if row.StoreID != storeID || row.CategoryID != categoryID || row.FixedValue != fixedValue {
		t.Fatalf("row optionals = %+v, want preserved numeric values", row)
	}

	converted := row.toPricingRule()
	if converted.StoreID == nil || *converted.StoreID != storeID {
		t.Fatalf("converted storeID = %v, want %d", converted.StoreID, storeID)
	}
	if converted.CategoryID == nil || *converted.CategoryID != categoryID {
		t.Fatalf("converted categoryID = %v, want %d", converted.CategoryID, categoryID)
	}
	if converted.FixedValue == nil || *converted.FixedValue != fixedValue {
		t.Fatalf("converted fixedValue = %v, want %v", converted.FixedValue, fixedValue)
	}
	if converted.Description != "desc" || converted.Remark != "note" {
		t.Fatalf("converted = %+v, want trimmed values preserved", converted)
	}
}
