package listingadmin

import (
	"encoding/json"
	"testing"
)

func TestApplyProductDataAuditFieldsSetsOwnerAndAuditColumns(t *testing.T) {
	t.Parallel()

	row := listingProductData{}
	applyProductDataAuditFields(&row, "user-1", true)

	if row.OwnerUserID != "user-1" {
		t.Fatalf("ownerUserID = %q, want user-1", row.OwnerUserID)
	}
	if row.Creator != "user-1" || row.CreatedBy != "user-1" {
		t.Fatalf("creator/createdBy = %q/%q, want user-1", row.Creator, row.CreatedBy)
	}
	if row.Updater != "user-1" || row.UpdatedBy != "user-1" {
		t.Fatalf("updater/updatedBy = %q/%q, want user-1", row.Updater, row.UpdatedBy)
	}
}

func TestListingProductDataConversionPreservesJSONAndPointers(t *testing.T) {
	t.Parallel()

	importTaskID := int64(1001)
	storeID := int64(11)
	shelfStatus := 2
	product := &ProductData{
		TenantID:     101,
		ImportTaskID: &importTaskID,
		StoreID:      &storeID,
		ProductID:    "B001",
		ImageURLs:    json.RawMessage(`["https://example.test/1.jpg"]`),
		Attributes:   json.RawMessage(`{"color":"white"}`),
		PlatformData: json.RawMessage(`{"spu":"123"}`),
		ShelfStatus:  &shelfStatus,
		Platform:     "SHEIN",
		Region:       "US",
		Title:        "Cotton shirt",
	}

	row := listingProductDataFromProductData(product)
	if row.ImportTaskID != 1001 || row.StoreID != 11 || row.ShelfStatus != 2 {
		t.Fatalf("row ids/status = %+v, want pointer values copied", row)
	}
	if row.ImageURLs != `["https://example.test/1.jpg"]` || row.Attributes != `{"color":"white"}` || row.PlatformData != `{"spu":"123"}` {
		t.Fatalf("row json fields = %q / %q / %q", row.ImageURLs, row.Attributes, row.PlatformData)
	}

	roundTrip := row.toProductData()
	if roundTrip.ImportTaskID == nil || *roundTrip.ImportTaskID != 1001 {
		t.Fatalf("roundTrip importTaskID = %+v, want 1001", roundTrip.ImportTaskID)
	}
	if string(roundTrip.ImageURLs) != `["https://example.test/1.jpg"]` {
		t.Fatalf("roundTrip imageURLs = %s", string(roundTrip.ImageURLs))
	}
	if string(roundTrip.Attributes) != `{"color":"white"}` {
		t.Fatalf("roundTrip attributes = %s", string(roundTrip.Attributes))
	}
}
