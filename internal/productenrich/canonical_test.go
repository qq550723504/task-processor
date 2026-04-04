package productenrich

import "testing"

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
	if canonical.Attributes["color"].Trace.Confidence <= 0 {
		t.Fatal("attribute confidence should be populated")
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
