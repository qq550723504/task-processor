package listingkit

import "testing"

func TestAttachRevisionHistoryStoreResolutionDecoratesItems(t *testing.T) {
	t.Parallel()

	page := &ListingKitRevisionHistoryPage{
		Items: []ListingKitRevisionRecord{{Platform: "shein"}},
	}
	storeResolution := &SheinStoreResolutionSummary{StoreID: 903}

	got := attachRevisionHistoryStoreResolution(page, storeResolution)
	if got == nil || got.Items[0].StoreResolution == nil || got.Items[0].StoreResolution.StoreID != 903 {
		t.Fatalf("page = %+v, want attached store resolution", got)
	}
}

func TestAttachRevisionHistoryDetailStoreResolutionDecoratesRecord(t *testing.T) {
	t.Parallel()

	detail := &ListingKitRevisionHistoryDetail{
		Record: &ListingKitRevisionRecord{Platform: "shein"},
	}
	storeResolution := &SheinStoreResolutionSummary{StoreID: 903}

	got := attachRevisionHistoryDetailStoreResolution(detail, storeResolution)
	if got == nil || got.Record == nil || got.Record.StoreResolution == nil || got.Record.StoreResolution.StoreID != 903 {
		t.Fatalf("detail = %+v, want attached store resolution", got)
	}
}
