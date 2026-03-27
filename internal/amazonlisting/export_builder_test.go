package amazonlisting

import "testing"

func TestExportBuilderBuildsListingsAPIRequest(t *testing.T) {
	builder := NewExportBuilder()

	draft := &AmazonListingDraft{
		TaskID:       "task-123",
		Marketplace:  "amazon",
		Country:      "US",
		Language:     "en_US",
		ProductType:  "Kitchen",
		Title:        "Ceramic Coffee Mug 12oz",
		Brand:        "Acme",
		Description:  "A ceramic mug for coffee, tea, and daily kitchen use.",
		BulletPoints: []string{"12oz capacity", "Comfortable handle", "Suitable for hot and cold drinks"},
		SearchTerms:  []string{"ceramic mug", "coffee cup"},
		Attributes: map[string]string{
			"material": "ceramic",
			"color":    "white",
		},
		Dimensions: &AmazonDimensions{Length: 10, Width: 8, Height: 9, Unit: "cm"},
		Weight:     &AmazonWeight{Value: 0.4, Unit: "kg"},
		Images: &AmazonImageBundle{
			MainImage:     "https://example.com/main.jpg",
			GalleryImages: []string{"https://example.com/gallery1.jpg", "https://example.com/gallery2.jpg"},
		},
		Pricing: &AmazonPricingDraft{Currency: "USD", SuggestedPrice: 19.99},
		Variants: []AmazonVariantDraft{
			{SKU: "MUG-001", Inventory: 25, IsDefault: true},
		},
	}

	export := builder.Build(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
		Language:    "en_US",
	}, draft)

	if export == nil || export.ListingsAPI == nil {
		t.Fatalf("expected listings api export")
	}
	if export.ListingsAPI.MarketplaceID != "ATVPDKIKX0DER" {
		t.Fatalf("unexpected marketplace id: %s", export.ListingsAPI.MarketplaceID)
	}
	if export.ListingsAPI.ProductType != "KITCHEN" {
		t.Fatalf("unexpected product type: %s", export.ListingsAPI.ProductType)
	}
	if export.ListingsAPI.SKU != "MUG-001" {
		t.Fatalf("unexpected sku: %s", export.ListingsAPI.SKU)
	}
	if export.ListingsAPI.ValidationPreviewRequest == nil {
		t.Fatalf("expected validation preview payload")
	}
	if export.ListingsAPI.CreateRequest == nil {
		t.Fatalf("expected create request payload")
	}
	if export.ListingsAPI.UpdateRequest == nil {
		t.Fatalf("expected update request payload")
	}
	if export.ListingsAPI.Patch == nil || len(export.ListingsAPI.Patch.Patches) == 0 {
		t.Fatalf("expected patch payload")
	}
	if _, ok := export.ListingsAPI.Attributes["item_name"]; !ok {
		t.Fatalf("expected item_name attribute")
	}
	if _, ok := export.ListingsAPI.Attributes["purchasable_offer"]; !ok {
		t.Fatalf("expected purchasable_offer attribute")
	}
	if _, ok := export.ListingsAPI.Attributes["main_product_image_locator"]; !ok {
		t.Fatalf("expected main image attribute")
	}
	if _, ok := export.ListingsAPI.Attributes["material"]; !ok {
		t.Fatalf("expected material attribute")
	}
}

func TestExportBuilderFallsBackToGeneratedSKUAndProductType(t *testing.T) {
	builder := NewExportBuilder()

	export := builder.Build(&GenerateRequest{
		Marketplace: "amazon",
		Country:     "US",
	}, &AmazonListingDraft{
		TaskID:       "task 456",
		CategoryPath: []string{"Home & Kitchen", "Drinkware"},
		Title:        "Minimal Mug",
		Brand:        "Acme",
	})

	if export == nil || export.ListingsAPI == nil {
		t.Fatalf("expected export payload")
	}
	if export.ListingsAPI.SKU == "" {
		t.Fatalf("expected generated sku")
	}
	if export.ListingsAPI.ProductType != "DRINKWARE" {
		t.Fatalf("expected category fallback product type, got %s", export.ListingsAPI.ProductType)
	}
}
