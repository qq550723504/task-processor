package listingadmin

import "testing"

func TestApplyProfitRuleDefaultsSetsMultipliers(t *testing.T) {
	t.Parallel()

	row := listingProfitRule{}
	applyProfitRuleDefaults(&row)

	if row.SalePriceMultiplier != 1 {
		t.Fatalf("salePriceMultiplier = %v, want 1", row.SalePriceMultiplier)
	}
	if row.DiscountPriceMultiplier != 1 {
		t.Fatalf("discountPriceMultiplier = %v, want 1", row.DiscountPriceMultiplier)
	}
}

func TestApplyProfitRuleAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingProfitRule{}
	applyProfitRuleAuditFields(&row, "user-1", true)

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

func TestListingProfitRuleConversionPreservesOptionalFields(t *testing.T) {
	t.Parallel()

	storeID := int64(11)
	categoryID := int64(22)
	rule := ProfitRule{
		ID:                      1,
		TenantID:                101,
		Name:                    " Margin ",
		RuleCode:                " PR-1 ",
		Description:             " desc ",
		StoreID:                 &storeID,
		CategoryID:              &categoryID,
		SalePriceMultiplier:     2.3,
		DiscountPriceMultiplier: 1.8,
		Status:                  2,
		Remark:                  " note ",
	}

	row := listingProfitRuleFromProfitRule(&rule)
	if row.Name != "Margin" || row.RuleCode != "PR-1" {
		t.Fatalf("trimmed row = %+v, want trimmed strings", row)
	}
	if row.StoreID != storeID || row.CategoryID != categoryID {
		t.Fatalf("row optionals = %+v, want preserved numeric values", row)
	}

	converted := row.toProfitRule()
	if converted.StoreID == nil || *converted.StoreID != storeID {
		t.Fatalf("converted storeID = %v, want %d", converted.StoreID, storeID)
	}
	if converted.CategoryID == nil || *converted.CategoryID != categoryID {
		t.Fatalf("converted categoryID = %v, want %d", converted.CategoryID, categoryID)
	}
	if converted.Description != "desc" || converted.Remark != "note" {
		t.Fatalf("converted = %+v, want trimmed values preserved", converted)
	}
}
