package listingkit

func appendRevisionHistory(result *ListingKitResult, record ListingKitRevisionRecord) {
	if result == nil {
		return
	}
	if record.RevisionID == "" {
		record.RevisionID = newRevisionHistoryRecordID()
	}
	result.RevisionHistoryTotal++
	result.RevisionHistory = append(result.RevisionHistory, record)
	result.RevisionHistory = applyRevisionHistoryRetention(result.RevisionHistory)
}
