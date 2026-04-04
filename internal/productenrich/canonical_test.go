package productenrich

import (
	"strings"
	"testing"
)

func TestBuildCanonicalProduct_WithMixedInputSources(t *testing.T) {
	req := &GenerateRequest{
		Text:       "portable blender",
		ImageURLs:  []string{"https://example.com/1.jpg"},
		ProductURL: "https://detail.1688.com/offer/123.html",
	}
	product := &ProductJSON{
		Title:         "Portable Blender Bottle",
		Category:      []string{"Kitchen", "Blenders"},
		Attributes:    map[string]string{"brand": "MixGo", "color": "white"},
		Description:   "USB rechargeable portable blender for smoothies.",
		SellingPoints: []string{"USB charging", "Travel friendly"},
		SEOKeywords:   []string{"portable blender"},
		Images:        []string{"https://example.com/1.jpg", "https://example.com/2.jpg"},
		Variants: []ProductVariant{
			{
				SKU:        "BLENDER-WHITE",
				Attributes: map[string]string{"color": "white"},
				Images:     []string{"https://example.com/variant.jpg"},
				IsDefault:  true,
			},
		},
		Evidence: map[string][]CanonicalSource{
			"title": {
				{Type: CanonicalSourceScrapedData, Detail: `scraped title: "Portable Blender Bottle"`},
			},
			"attributes.color": {
				{Type: CanonicalSourceScrapedData, Detail: "scraped spec color: white"},
			},
		},
	}

	canonical := BuildCanonicalProduct(req, product)
	if canonical == nil {
		t.Fatal("canonical product should not be nil")
	}
	if canonical.Title != product.Title {
		t.Fatalf("Title = %q, want %q", canonical.Title, product.Title)
	}
	if canonical.Brand != "MixGo" {
		t.Fatalf("Brand = %q, want MixGo", canonical.Brand)
	}
	if len(canonical.Images) != 2 {
		t.Fatalf("len(Images) = %d, want 2", len(canonical.Images))
	}
	if canonical.Images[0].Role != "primary" {
		t.Fatalf("first image role = %q, want primary", canonical.Images[0].Role)
	}
	if !hasSourceType(canonical.FieldTraces["title"].Sources, CanonicalSourceLLM) {
		t.Fatal("title trace should include llm source")
	}
	if !hasSourceType(canonical.FieldTraces["title"].Sources, CanonicalSourceProductURL) {
		t.Fatal("title trace should include product_url source")
	}
	if got := canonical.FieldTraces["title"].Sources[0].Detail; !strings.Contains(got, `user input: "portable blender"`) {
		t.Fatalf("user text detail = %q", got)
	}
	if got := canonical.FieldTraces["title"].Sources[1].Detail; !strings.Contains(got, "user image: https://example.com/1.jpg") {
		t.Fatalf("image detail = %q", got)
	}
	if got := canonical.FieldTraces["title"].Sources[3].Detail; got != "normalized from product page: https://detail.1688.com/offer/123.html" {
		t.Fatalf("scraped detail = %q", got)
	}
	if got := canonical.FieldTraces["title"].Sources[4].Detail; got != "LLM-generated product normalization" {
		t.Fatalf("llm detail = %q", got)
	}
	if got := canonical.FieldTraces["title"].Sources[5].Detail; got != `scraped title: "Portable Blender Bottle"` {
		t.Fatalf("title evidence detail = %q", got)
	}
	if canonical.Attributes["color"].Trace.Confidence <= 0 {
		t.Fatal("attribute confidence should be populated")
	}
	if got := canonical.Attributes["color"].Trace.Sources[len(canonical.Attributes["color"].Trace.Sources)-1].Detail; got != "scraped spec color: white" {
		t.Fatalf("attribute evidence detail = %q", got)
	}
	if len(canonical.Variants) != 1 {
		t.Fatalf("len(Variants) = %d, want 1", len(canonical.Variants))
	}
}

func TestBuildCanonicalProduct_EmptyCriticalFieldsNeedReview(t *testing.T) {
	canonical := BuildCanonicalProduct(&GenerateRequest{Text: "sample"}, &ProductJSON{})
	if canonical == nil {
		t.Fatal("canonical product should not be nil")
	}
	if !canonical.NeedsReview {
		t.Fatal("expected NeedsReview for empty title/description")
	}
}
