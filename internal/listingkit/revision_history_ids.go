package listingkit

import (
	"fmt"

	"github.com/google/uuid"
)

func newRevisionHistoryRecordID() string {
	return uuid.NewString()
}

func revisionHistoryRecordID(record ListingKitRevisionRecord, index int) string {
	if record.RevisionID != "" {
		return record.RevisionID
	}
	return fmt.Sprintf("legacy-%d-%d", record.UpdatedAt.UTC().UnixNano(), index)
}

func withRevisionHistoryRecordID(record ListingKitRevisionRecord, index int) ListingKitRevisionRecord {
	record.RevisionID = revisionHistoryRecordID(record, index)
	return withRevisionTimelineSummary(record)
}
