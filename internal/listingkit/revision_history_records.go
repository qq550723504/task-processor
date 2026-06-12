package listingkit

func buildRevisionHistoryRecords(records []ListingKitRevisionRecord) []ListingKitRevisionRecord {
	if len(records) == 0 {
		return nil
	}
	items := make([]ListingKitRevisionRecord, 0, len(records))
	for i, record := range records {
		items = append(items, withRevisionHistoryRecordID(record, i))
	}
	return items
}

func buildAdjacentRevisionID(records []ListingKitRevisionRecord, index int) string {
	if index < 0 || index >= len(records) {
		return ""
	}
	return records[index].RevisionID
}
