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

func TestValidatorFlagsMediumContentIPRiskForBrandReference(t *testing.T) {
	validator := NewValidator()
	draft := &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Running Shoes Street Style",
		Description:  "Comfortable shoes with breathable mesh for everyday wear and sports use without direct compatibility claims.",
		CategoryPath: []string{"Clothing, Shoes & Jewelry", "Shoes"},
		Brand:        "Acme",
		BulletPoints: []string{"Nike style inspired silhouette", "Breathable upper", "Lightweight outsole"},
		Pricing:      &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
		},
	}

	report := validator.Validate(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
		Options:     &GenerateOptions{ProcessImages: false},
	}, draft)

	if !report.Ready {
		t.Fatalf("expected medium ip risk to remain reviewable, got blocking issues: %+v", report.BlockingIssues)
	}
	if !report.NeedsReview {
		t.Fatal("expected needs review for brand reference")
	}
	if draft.IPRisk == nil || draft.IPRisk.Level != "medium" {
		t.Fatalf("expected medium ip risk report, got %+v", draft.IPRisk)
	}
}

func TestValidatorBlocksHighContentIPRiskForCompatibilityPhrase(t *testing.T) {
	validator := NewValidator()
	draft := &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Replacement Brush Head Compatible with Dyson V8",
		Description:  "High performance replacement accessory for vacuum cleaner maintenance and home use.",
		CategoryPath: []string{"Home & Kitchen", "Cleaning Supplies"},
		Brand:        "Acme",
		BulletPoints: []string{"Compatible with Dyson V8", "Replacement for old brush head", "Easy to install"},
		Pricing:      &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
		},
	}

	report := validator.Validate(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
		Options:     &GenerateOptions{ProcessImages: false},
	}, draft)

	if report.Ready {
		t.Fatal("expected high IP risk to block readiness")
	}
	if draft.IPRisk == nil || draft.IPRisk.Level != "high" {
		t.Fatalf("expected high ip risk report, got %+v", draft.IPRisk)
	}
}

func TestValidatorDoesNotFlagOwnedBrandReference(t *testing.T) {
	validator := NewValidator()
	draft := &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Acme Running Shoes",
		Description:  "Acme branded running shoes with breathable mesh upper and cushioned outsole for daily training.",
		CategoryPath: []string{"Clothing, Shoes & Jewelry", "Shoes"},
		Brand:        "Acme",
		BulletPoints: []string{"Acme logo on tongue", "Breathable upper", "Lightweight outsole"},
		Pricing:      &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
		},
	}

	report := validator.Validate(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
		BrandHint:   "Acme",
		Options:     &GenerateOptions{ProcessImages: false},
	}, draft)

	if draft.IPRisk != nil {
		t.Fatalf("expected owned brand reference not to be flagged, got %+v", draft.IPRisk)
	}
	if !report.Ready {
		t.Fatalf("expected report ready, got blocking issues %+v", report.BlockingIssues)
	}
}

func TestValidatorEscalatesListingIPRiskWhenImageRiskExists(t *testing.T) {
	validator := NewValidator()
	draft := &AmazonListingDraft{
		Marketplace:  "amazon",
		Country:      "US",
		Title:        "Ceramic Mug for Coffee",
		Description:  "A ceramic mug with durable finish and comfortable handle for home and office beverage use.",
		CategoryPath: []string{"Home & Kitchen"},
		Brand:        "Acme",
		BulletPoints: []string{"Durable ceramic body", "Comfortable handle", "Suitable for coffee and tea"},
		Pricing:      &AmazonPricingDraft{Currency: "USD"},
		Variants: []AmazonVariantDraft{
			{SKU: "SKU-1", IsDefault: true},
		},
		ListingIPRisk: &IPRiskReport{
			Level:   "medium",
			Score:   0.4,
			Reasons: []string{"image contains logo or watermark risk"},
		},
	}

	report := validator.Validate(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
		Options:     &GenerateOptions{ProcessImages: false},
	}, draft)

	if !report.NeedsReview {
		t.Fatal("expected merged listing ip risk to require review")
	}
	if draft.ListingIPRisk == nil || draft.ListingIPRisk.Level != "medium" {
		t.Fatalf("expected medium listing ip risk, got %+v", draft.ListingIPRisk)
	}
}
