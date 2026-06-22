package listingkit

import (
	"slices"
	"testing"

	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSheinSourceProductSummary(t *testing.T) {
	t.Parallel()

	product := &canonical.Product{
		Title:        "Bottle",
		CategoryPath: []string{"Home", "Kitchen"},
		Attributes: map[string]canonical.Attribute{
			"sku":   {Value: "SKU-1"},
			"brand": {Value: "Acme"},
		},
	}

	summary := sheinworkspace.BuildSourceProductSummary(product)
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Title != "Bottle" || summary.SKU != "SKU-1" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary.Attributes["brand"] != "Acme" {
		t.Fatalf("attributes = %+v", summary.Attributes)
	}
}

func TestBuildSheinPreviewReviewSummary(t *testing.T) {
	t.Parallel()

	needsReview, summary := sheinworkspace.BuildPreviewReviewSummary(&SheinPackage{
		ReviewNotes: []string{"缺少类目", "缺少类目"},
		Inspection: &sheinpub.Inspection{
			NeedsReview: true,
			Summary:     []string{"图片待确认", "缺少类目"},
		},
	})
	if !needsReview {
		t.Fatal("needsReview = false, want true")
	}
	want := []string{"缺少类目", "图片待确认"}
	if !slices.Equal(summary, want) {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
}
