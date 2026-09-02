package amazonlisting

import "testing"

func TestValidatorBlocksMissingRequiredDraftFacts(t *testing.T) {
	report := NewValidator().Validate(&GenerateRequest{Marketplace: "amazon", Country: "US"}, &AmazonListingDraft{
		Marketplace: "amazon",
		Country:     "US",
		Pricing:     &AmazonPricingDraft{Currency: "USD"},
		Variants:    []AmazonVariantDraft{{SKU: "SKU-1", IsDefault: true}},
	})
	if report.Ready || len(report.BlockingIssues) == 0 {
		t.Fatalf("validation report = %+v, want blocking issues", report)
	}
}

func TestValidatorKeepsWarningsReviewable(t *testing.T) {
	report := NewValidator().Validate(&GenerateRequest{Marketplace: "amazon", Country: "US"}, &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Short title for mug",
		Description:  "This ceramic mug is suitable for coffee and tea use in daily settings with a useful handle.",
		CategoryPath: []string{"Home & Kitchen", "Kitchen & Dining"},
		Brand:        "Generic",
		BulletPoints: []string{"One bullet only"},
		Images:       &AmazonImageBundle{MainImage: "https://cdn.example.com/main.jpg"},
		Pricing:      &AmazonPricingDraft{Currency: "USD"},
		Variants:     []AmazonVariantDraft{{SKU: "SKU-1", IsDefault: true}},
	})
	if !report.Ready || !report.NeedsReview || len(report.Warnings) == 0 {
		t.Fatalf("validation report = %+v, want ready with review warnings", report)
	}
}

func TestValidatorBlocksDuplicateVariantSKU(t *testing.T) {
	report := NewValidator().Validate(&GenerateRequest{Marketplace: "amazon", Country: "US"}, &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Ceramic Coffee Mug 12oz with Handle",
		Description:  "A ceramic mug with durable finish and comfortable handle for home and office beverage use.",
		CategoryPath: []string{"Home & Kitchen"},
		Brand:        "Acme",
		BulletPoints: []string{"Durable ceramic body", "Comfortable handle", "Suitable for coffee and tea"},
		Images:       &AmazonImageBundle{MainImage: "https://cdn.example.com/main.jpg"},
		Pricing:      &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
			{SKU: "SKU-1"},
		},
	})
	if report.Ready || len(report.BlockingIssues) == 0 {
		t.Fatalf("validation report = %+v, want duplicate SKU failure", report)
	}
}

func TestValidatorConsumesStructuredListingIPRisk(t *testing.T) {
	draft := &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Ceramic Mug for Coffee",
		Description:  "A ceramic mug with durable finish and comfortable handle for home and office beverage use.",
		CategoryPath: []string{"Home & Kitchen"},
		Brand:        "Acme",
		BulletPoints: []string{"Durable ceramic body", "Comfortable handle", "Suitable for coffee and tea"},
		Images:       &AmazonImageBundle{MainImage: "https://cdn.example.com/main.jpg"},
		Pricing:      &AmazonPricingDraft{Currency: "USD"},
		Variants:     []AmazonVariantDraft{{SKU: "SKU-1", IsDefault: true}},
		ListingIPRisk: &IPRiskReport{
			Level:   "medium",
			Score:   0.4,
			Reasons: []string{"structured review finding"},
		},
	}
	report := NewValidator().Validate(&GenerateRequest{Marketplace: "amazon", Country: "US"}, draft)
	if !report.NeedsReview || draft.ListingIPRisk.Level != "medium" {
		t.Fatalf("validation report = %+v risk = %+v", report, draft.ListingIPRisk)
	}
}
