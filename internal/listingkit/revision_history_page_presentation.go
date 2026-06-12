package listingkit

import "time"

func buildRevisionHistoryPageMetaValue(result *ListingKitResult, items []ListingKitRevisionRecord, hasMore bool, actionType string) *ListingKitRevisionHistoryPageMeta {
	if result == nil {
		return nil
	}
	total := result.RevisionHistoryTotal
	if total == 0 && len(result.RevisionHistory) > 0 {
		total = len(result.RevisionHistory)
	}
	meta := &ListingKitRevisionHistoryPageMeta{
		TotalRecords:    total,
		ReturnedRecords: len(items),
		HasMore:         hasMore,
		IsTruncated:     total > len(result.RevisionHistory),
		MaxRecords:      maxRevisionHistoryRecords,
		ActionType:      actionType,
		Counts:          buildRevisionHistoryCountsValue(result.RevisionHistory),
	}
	if hasMore && len(items) > 0 {
		meta.NextBefore = items[len(items)-1].UpdatedAt.Format(time.RFC3339)
	}
	return meta
}

func buildRevisionHistoryCountsValue(records []ListingKitRevisionRecord) *ListingKitRevisionHistoryCounts {
	if len(records) == 0 {
		return &ListingKitRevisionHistoryCounts{}
	}
	counts := &ListingKitRevisionHistoryCounts{}
	for _, hydrated := range buildRevisionHistoryRecords(records) {
		counts.All++
		switch hydrated.ActionType {
		case RevisionActionTypeRestore:
			counts.Restore++
		default:
			counts.Edit++
		}
	}
	return counts
}
