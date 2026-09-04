package listingkit

import "testing"

func TestSheinStoreResolutionSummaryFromTaskUsesSnapshot(t *testing.T) {
	t.Parallel()

	summary := sheinStoreResolutionSummaryFromTask(&Task{TenantID: "tenant-test", SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
		StoreID: 903,
		Site:    "GB",
	},
	})
	if summary == nil || summary.StoreID != 903 || summary.Site != "GB" {
		t.Fatalf("summary = %+v, want snapshot-backed summary", summary)
	}
}

func TestSheinSubmissionStoreResolutionFromTaskUsesSnapshot(t *testing.T) {
	t.Parallel()

	resolution := sheinSubmissionStoreResolutionFromTask(&Task{TenantID: "tenant-test", SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
		StoreID: 903,
		Site:    "GB",
	},
	})
	if resolution == nil || resolution.StoreID != 903 || resolution.Site != "GB" {
		t.Fatalf("resolution = %+v, want snapshot-backed submission resolution", resolution)
	}
}
