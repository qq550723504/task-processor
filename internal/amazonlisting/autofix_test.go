package amazonlisting

import (
	"strings"
	"testing"
)

func TestAutoFixerFixesTitleBrandBulletsAndVariants(t *testing.T) {
	fixer := NewAutoFixer()
	draft := &AmazonListingDraft{
		Country:     "US",
		ProductType: "Ceramic Mug",
		Title:       strings.Repeat("A", 220),
		Description: "Dishwasher safe. Comfortable handle. Good for coffee and tea.",
		SearchTerms: []string{"kitchen accessory", "gift ready"},
		Attributes: map[string]string{
			"material": "ceramic",
		},
		Pricing: &AmazonPricingDraft{},
		Variants: []AmazonVariantDraft{
			{},
		},
		Images: &AmazonImageBundle{
			RawInputImages: []string{"https://example.com/raw.jpg"},
		},
	}

	fixer.Fix(&GenerateRequest{BrandHint: "Acme", Country: "US"}, draft)

	if len([]rune(draft.Title)) != 200 {
		t.Fatalf("expected title to be trimmed to 200 chars, got %d", len([]rune(draft.Title)))
	}
	if draft.Brand != "Acme" {
		t.Fatalf("expected brand to be filled from hint")
	}
	if len(draft.BulletPoints) < 3 {
		t.Fatalf("expected bullet points to be auto-filled")
	}
	if draft.Pricing == nil || draft.Pricing.Currency != "USD" {
		t.Fatalf("expected pricing currency to be set to USD")
	}
	if len(draft.Variants) != 1 || draft.Variants[0].SKU == "" || !draft.Variants[0].IsDefault {
		t.Fatalf("expected variant SKU/default to be auto-fixed")
	}
	if draft.Images.MainImage == "" {
		t.Fatalf("expected main image to fall back from raw input")
	}
}
