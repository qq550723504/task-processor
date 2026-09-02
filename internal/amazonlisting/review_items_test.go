package amazonlisting

import (
	"testing"

	"task-processor/internal/product/catalog"
)

func TestBuildReviewItemsFromSnapshotIncludesTraceEvidence(t *testing.T) {
	snapshot := &catalog.ProductSnapshot{Attributes: []catalog.Attribute{{
		Name:  "material",
		Value: "steel",
		Trace: catalog.Trace{
			Sources:     []catalog.SourceRecord{{Type: "scraped_data", Detail: "specification.material"}},
			Confidence:  0.62,
			IsInferred:  true,
			NeedsReview: true,
		},
	}}}

	items := buildReviewItemsFromSnapshot(snapshot)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.Field != "attributes.material" || item.Source != "scraped_data" {
		t.Fatalf("review item = %+v", item)
	}
	if item.Confidence != 0.62 || !item.IsInferred {
		t.Fatalf("trace metadata = %+v", item)
	}
	if len(item.Evidence) != 1 || item.Evidence[0].Detail != "specification.material" {
		t.Fatalf("evidence = %+v", item.Evidence)
	}
}
