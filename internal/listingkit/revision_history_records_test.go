package listingkit

import (
	"testing"
	"time"
)

func TestBuildRevisionHistoryRecordsHydratesIDs(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	items := buildRevisionHistoryRecords([]ListingKitRevisionRecord{
		{Platform: "shein", UpdatedAt: now},
	})
	if len(items) != 1 || items[0].RevisionID == "" {
		t.Fatalf("items = %+v, want hydrated revision id", items)
	}
	if items[0].Timeline == nil {
		t.Fatalf("items = %+v, want hydrated timeline", items)
	}
}
