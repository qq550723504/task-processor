package listingadmin

import "testing"

func TestApplyCategoryDefaultsSetsLevel(t *testing.T) {
	t.Parallel()

	row := listingCategory{}
	applyCategoryDefaults(&row)

	if row.Level != 1 {
		t.Fatalf("level = %d, want 1", row.Level)
	}
}

func TestApplyCategoryAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingCategory{}
	applyCategoryAuditFields(&row, "user-1", true)

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

func TestListingCategoryConversionPreservesFields(t *testing.T) {
	t.Parallel()

	category := Category{
		ID:          1,
		TenantID:    101,
		Name:        " Apparel ",
		Code:        " APPAREL ",
		ParentID:    10,
		Level:       2,
		Sort:        3,
		Icon:        " shirt ",
		Image:       " /img/a.png ",
		Description: " desc ",
		Status:      1,
	}

	row := listingCategoryFromCategory(&category)
	if row.Name != "Apparel" || row.Code != "APPAREL" || row.Icon != "shirt" {
		t.Fatalf("trimmed row = %+v, want trimmed strings", row)
	}

	converted := row.toCategory()
	if converted.ParentID != 10 || converted.Level != 2 || converted.Sort != 3 {
		t.Fatalf("converted = %+v, want numeric fields preserved", converted)
	}
	if converted.Description != "desc" || converted.Image != "/img/a.png" {
		t.Fatalf("converted = %+v, want trimmed values preserved", converted)
	}
}
