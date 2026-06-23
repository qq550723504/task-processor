package listingadmin

import "testing"

func TestApplyImportTaskDefaultsFillsSourceRegionPriorityAndRetry(t *testing.T) {
	t.Parallel()

	row := listingProductImportTask{
		Platform: "Amazon",
	}
	applyImportTaskDefaults(&row)

	if row.SourcePlatform != "Amazon" {
		t.Fatalf("sourcePlatform = %q, want Amazon", row.SourcePlatform)
	}
	if row.Region != "US" {
		t.Fatalf("region = %q, want US", row.Region)
	}
	if row.Priority != 5 {
		t.Fatalf("priority = %d, want 5", row.Priority)
	}
	if row.MaxRetryCount != 3 {
		t.Fatalf("maxRetryCount = %d, want 3", row.MaxRetryCount)
	}
}

func TestApplyImportTaskAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingProductImportTask{}
	applyImportTaskAuditFields(&row, "user-1", true)

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

func TestListingProductImportTaskConversionPreservesPointersAndDefaults(t *testing.T) {
	t.Parallel()

	storeID := int64(11)
	categoryID := int64(22)
	task := ImportTask{
		ID:             1,
		TenantID:       101,
		StoreID:        &storeID,
		Platform:       " Amazon ",
		TargetPlatform: " SHEIN ",
		SourcePlatform: " ",
		Region:         " US ",
		CategoryID:     &categoryID,
		ProductID:      " B001 ",
		Status:         2,
		ErrorMessage:   " bad ",
		ReasonCode:     " reason ",
		Stage:          " normalize ",
		RetryCount:     1,
		MaxRetryCount:  4,
		Remark:         " note ",
		Priority:       9,
		Creator:        " creator-1 ",
		Updater:        " updater-1 ",
	}

	row := listingProductImportTaskFromImportTask(task)
	if row.SourcePlatform != "Amazon" {
		t.Fatalf("sourcePlatform = %q, want Amazon fallback", row.SourcePlatform)
	}
	if row.Platform != "Amazon" || row.Region != "US" || row.ProductID != "B001" {
		t.Fatalf("trimmed row = %+v, want trimmed platform/region/productID", row)
	}
	if row.Creator != "creator-1" || row.Updater != "updater-1" {
		t.Fatalf("row creator/updater = %q/%q, want creator-1/updater-1", row.Creator, row.Updater)
	}

	converted := row.toImportTask()
	if converted.StoreID == nil || *converted.StoreID != storeID {
		t.Fatalf("converted storeID = %v, want %d", converted.StoreID, storeID)
	}
	if converted.CategoryID == nil || *converted.CategoryID != categoryID {
		t.Fatalf("converted categoryID = %v, want %d", converted.CategoryID, categoryID)
	}
	if converted.TargetPlatform != "SHEIN" || converted.ErrorMessage != "bad" || converted.Remark != "note" {
		t.Fatalf("converted = %+v, want trimmed values preserved", converted)
	}
	if converted.Creator != "creator-1" || converted.Updater != "updater-1" {
		t.Fatalf("converted creator/updater = %q/%q, want creator-1/updater-1", converted.Creator, converted.Updater)
	}
}
