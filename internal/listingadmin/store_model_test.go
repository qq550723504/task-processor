package listingadmin

import "testing"

func TestListingStoreToStoreUsesAuditFallbacks(t *testing.T) {
	t.Parallel()

	row := listingStore{
		ID:          1,
		TenantID:    101,
		OwnerUserID: "",
		CreatedBy:   "creator-id",
		Creator:     "creator-name",
		UpdatedBy:   "",
		Updater:     "updater-name",
		Name:        "Demo Store",
		Username:    "demo-user",
		Password:    "secret",
		Platform:    "SHEIN",
		ShopType:    "semi",
		Region:      "US",
	}

	store := row.toStore()
	if store.OwnerUserID != "creator-id" {
		t.Fatalf("ownerUserID = %q, want creator-id fallback", store.OwnerUserID)
	}
	if store.CreatedBy != "creator-id" {
		t.Fatalf("createdBy = %q, want creator-id fallback", store.CreatedBy)
	}
	if store.UpdatedBy != "updater-name" {
		t.Fatalf("updatedBy = %q, want updater-name fallback", store.UpdatedBy)
	}
}

func TestApplyStoreCreateDefaultsFillsRegionDailyLimitTypeAndAuditNames(t *testing.T) {
	t.Parallel()

	row := listingStore{
		OwnerUserID: "owner-1",
		CreatedBy:   "created-1",
		UpdatedBy:   "updated-1",
	}

	applyStoreCreateDefaults(&row)

	if row.DailyLimitType != "SPU" {
		t.Fatalf("dailyLimitType = %q, want SPU", row.DailyLimitType)
	}
	if row.Region != "US" {
		t.Fatalf("region = %q, want US", row.Region)
	}
	if row.Creator != "created-1" {
		t.Fatalf("creator = %q, want created-1", row.Creator)
	}
	if row.Updater != "updated-1" {
		t.Fatalf("updater = %q, want updated-1", row.Updater)
	}
}
