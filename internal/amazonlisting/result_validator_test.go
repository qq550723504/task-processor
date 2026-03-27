package amazonlisting

import "testing"

func TestValidatorBlockingIssues(t *testing.T) {
	validator := NewValidator()
	report := validator.Validate(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
		Options:     &GenerateOptions{ProcessImages: true},
	}, &AmazonListingDraft{
		Marketplace: "amazon",
		Country:     "US",
		Pricing:     &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
		},
	})

	if report.Ready {
		t.Fatalf("expected report to be not ready")
	}
	if len(report.BlockingIssues) == 0 {
		t.Fatalf("expected blocking issues")
	}
}

func TestValidatorNeedsReviewWarnings(t *testing.T) {
	validator := NewValidator()
	report := validator.Validate(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
		Options:     &GenerateOptions{ProcessImages: true},
	}, &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Short title for mug",
		Description:  "This ceramic mug is suitable for coffee and tea use in daily settings with a useful handle.",
		CategoryPath: []string{"Home & Kitchen", "Kitchen & Dining"},
		Brand:        "Generic",
		BulletPoints: []string{"One bullet only"},
		Images: &AmazonImageBundle{
			MainImage: "https://example.com/main.jpg",
		},
		Pricing: &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
		},
	})

	if !report.Ready {
		t.Fatalf("expected report to remain ready with warnings only")
	}
	if !report.NeedsReview {
		t.Fatalf("expected needs review")
	}
	if len(report.Warnings) == 0 {
		t.Fatalf("expected warnings")
	}
}

func TestValidatorVariantDuplicatesBlock(t *testing.T) {
	validator := NewValidator()
	report := validator.Validate(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
	}, &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Ceramic Coffee Mug 12oz with Handle",
		Description:  "A ceramic mug with durable finish and comfortable handle for home and office beverage use.",
		CategoryPath: []string{"Home & Kitchen"},
		Brand:        "Acme",
		BulletPoints: []string{"Durable ceramic body", "Comfortable handle", "Suitable for coffee and tea"},
		Images: &AmazonImageBundle{
			MainImage:     "https://example.com/main.jpg",
			WhiteBgImage:  "https://example.com/white.jpg",
			GalleryImages: []string{"https://example.com/gallery1.jpg"},
		},
		Pricing: &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
			{SKU: "SKU-1"},
		},
	})

	if report.Ready {
		t.Fatalf("expected duplicate SKU to block readiness")
	}
	if len(report.BlockingIssues) == 0 {
		t.Fatalf("expected blocking issue for duplicate sku")
	}
}
