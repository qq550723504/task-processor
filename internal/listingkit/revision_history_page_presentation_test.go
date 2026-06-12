package listingkit

import (
	"testing"
	"time"
)

func TestBuildRevisionHistoryPageMetaValueBuildsCountsAndCursor(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	meta := buildRevisionHistoryPageMetaValue(&ListingKitResult{
		RevisionHistoryTotal: 3,
		RevisionHistory: []ListingKitRevisionRecord{
			{ActionType: RevisionActionTypeEdit, UpdatedAt: now.Add(-2 * time.Minute)},
			{ActionType: RevisionActionTypeRestore, UpdatedAt: now.Add(-time.Minute)},
		},
	}, []ListingKitRevisionRecord{
		{UpdatedAt: now},
	}, true, RevisionActionTypeRestore)
	if meta == nil {
		t.Fatal("meta = nil")
	}
	if meta.TotalRecords != 3 || meta.ReturnedRecords != 1 || !meta.HasMore || meta.NextBefore == "" {
		t.Fatalf("meta = %+v", meta)
	}
	if meta.Counts == nil || meta.Counts.All != 2 || meta.Counts.Edit != 1 || meta.Counts.Restore != 1 {
		t.Fatalf("counts = %+v", meta)
	}
}
