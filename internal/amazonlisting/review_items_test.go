package amazonlisting

import (
	"testing"

	"task-processor/internal/productenrich"
)

func TestBuildReviewItemsFromCanonicalIncludesTraceEvidence(t *testing.T) {
	product := &productenrich.CanonicalProduct{
		Title:       "Demo Product",
		Description: "Demo description",
		FieldTraces: map[string]productenrich.FieldTrace{
			"title": {
				Sources: []productenrich.CanonicalSource{
					{Type: productenrich.CanonicalSourceProductURL, Detail: "https://detail.1688.com/offer/123.html"},
					{Type: productenrich.CanonicalSourceScrapedData, Detail: "scraped_title"},
					{Type: productenrich.CanonicalSourceLLM, Detail: "productenrich_product_json"},
				},
				Confidence:  0.62,
				IsInferred:  true,
				NeedsReview: true,
			},
		},
		NeedsReview: true,
	}

	items := buildReviewItemsFromCanonical(product)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0]
	if item.Field != "title" {
		t.Fatalf("field = %q, want title", item.Field)
	}
	if item.Source != "product_url,scraped_data,llm" {
		t.Fatalf("source = %q", item.Source)
	}
	if item.Confidence != 0.62 {
		t.Fatalf("confidence = %v, want 0.62", item.Confidence)
	}
	if !item.IsInferred {
		t.Fatal("expected item to be inferred")
	}
	if len(item.Evidence) != 4 {
		t.Fatalf("len(evidence) = %d, want 4", len(item.Evidence))
	}
	if item.Evidence[0].Type != "product_url" || item.Evidence[0].Detail != "https://detail.1688.com/offer/123.html" {
		t.Fatalf("unexpected first evidence: %+v", item.Evidence[0])
	}
	if item.Evidence[3].Type != "field_value" || item.Evidence[3].Detail != `title = "Demo Product"` {
		t.Fatalf("unexpected field snippet evidence: %+v", item.Evidence[3])
	}
}
