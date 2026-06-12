package listingkit

func attachRevisionHistoryStoreResolution(
	page *ListingKitRevisionHistoryPage,
	storeResolution *SheinStoreResolutionSummary,
) *ListingKitRevisionHistoryPage {
	if page == nil || storeResolution == nil {
		return page
	}
	for idx := range page.Items {
		page.Items[idx].StoreResolution = storeResolution
	}
	return page
}

func attachRevisionHistoryDetailStoreResolution(
	detail *ListingKitRevisionHistoryDetail,
	storeResolution *SheinStoreResolutionSummary,
) *ListingKitRevisionHistoryDetail {
	if detail == nil || detail.Record == nil || storeResolution == nil {
		return detail
	}
	detail.Record.StoreResolution = storeResolution
	return detail
}
