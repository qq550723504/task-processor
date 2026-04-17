package listingkit

func buildRevisionHistoryRestorePayload(record *ListingKitRevisionRecord) *ApplyRevisionRequest {
	if record == nil {
		return nil
	}
	revisionID := record.RevisionID
	if revisionID == "" {
		return nil
	}
	return &ApplyRevisionRequest{
		Platform:              firstNonEmpty(record.Platform, "shein"),
		Actor:                 "desktop-client",
		Reason:                buildRevisionHistoryRestoreReason(record),
		RestoreFromRevisionID: revisionID,
	}
}
