package listingkit

const maxRevisionHistoryRecords = 20

func applyRevisionHistoryRetention(records []ListingKitRevisionRecord) []ListingKitRevisionRecord {
	if len(records) <= maxRevisionHistoryRecords {
		return records
	}
	start := len(records) - maxRevisionHistoryRecords
	trimmed := append([]ListingKitRevisionRecord(nil), records[start:]...)
	return trimmed
}

func buildRevisionHistoryMeta(result *ListingKitResult) *ListingKitRevisionHistoryMeta {
	if result == nil {
		return nil
	}
	total := result.RevisionHistoryTotal
	if total == 0 && len(result.RevisionHistory) > 0 {
		total = len(result.RevisionHistory)
	}
	return &ListingKitRevisionHistoryMeta{
		TotalRecords:    total,
		ReturnedRecords: len(result.RevisionHistory),
		HasMore:         total > len(result.RevisionHistory),
		IsTruncated:     total > len(result.RevisionHistory),
		MaxRecords:      maxRevisionHistoryRecords,
	}
}
