package listingkit

import (
	"context"
	"testing"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sheinmanaged "task-processor/internal/publishing/sheinmanaged"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

type stubAmazonDraftBuilder struct{}

type stubSheinCategoryAPI struct {
	info *sheincategory.CategoryInfo
}

type stubSheinAttributeAPI struct {
	templates *sheinattribute.AttributeTemplateInfo
}

type scriptedListingKitLLM struct {
	responses []string
}

func (s *scriptedListingKitLLM) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, nil
}

func (s *scriptedListingKitLLM) Generate(context.Context, string) (string, error) {
	if len(s.responses) == 0 {
		return "", nil
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, nil
}

func (s *scriptedListingKitLLM) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}

func (s *scriptedListingKitLLM) GetDefaultModel() string {
	return "test"
}

func (stubAmazonDraftBuilder) Build(req *GenerateRequest, canonical *canonical.Product, image *productimage.ImageProcessResult) *amazonlisting.AmazonListingDraft {
	return &amazonlisting.AmazonListingDraft{
		Marketplace: "amazon",
		Title:       canonical.Title,
		Brand:       canonical.Brand,
		Images: &amazonlisting.AmazonImageBundle{
			MainImage: canonical.Images[0].URL,
		},
	}
}

func (s stubSheinCategoryAPI) GetCategory(categoryID int) (*sheincategory.CategoryInfo, error) {
	if s.info == nil {
		return nil, nil
	}
	info := *s.info
	info.CategoryID = categoryID
	return &info, nil
}

func (s stubSheinCategoryAPI) GetCategoryTree() (*sheincategory.CategoryTreeResponse, error) {
	return nil, nil
}

func (s stubSheinCategoryAPI) SuggestCategoryByText(productInfo string) (*sheincategory.SuggestCategoryResponse, error) {
	return nil, nil
}

func (s stubSheinAttributeAPI) GetAttributeTemplates(categoryID int) (*sheinattribute.AttributeTemplateInfo, error) {
	return s.templates, nil
}

func (s stubSheinAttributeAPI) ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
	return nil, nil
}

func (s stubSheinAttributeAPI) AddCustomAttributeValue(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
	return nil, nil
}

func TestAssemblerAssembleBuildsPlatformPackages(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-1",
		Request: &GenerateRequest{
			Text:      "test product",
			Platforms: []string{"amazon", "shein", "temu", "walmart"},
			Country:   "US",
			Language:  "en_US",
			BrandHint: "DemoBrand",
		},
	}

	canonical := &canonical.Product{
		Title:         "Wireless Earbuds",
		Brand:         "Acme",
		CategoryPath:  []string{"Electronics", "Headphones"},
		Description:   "Noise cancelling earbuds",
		SellingPoints: []string{"ANC", "Bluetooth 5.3"},
		Specifications: &canonical.ProductSpecs{
			Dimensions: &canonical.Dimensions{
				Length: 12.5,
				Width:  8.2,
				Height: 4.1,
				Unit:   "cm",
			},
			Weight: &canonical.Weight{
				Value: 0.35,
				Unit:  "kg",
			},
		},
		Attributes: map[string]canonical.Attribute{
			"color": {Value: "Black"},
		},
		Images: []canonical.Image{
			{URL: "https://example.com/source-main.jpg"},
			{URL: "https://example.com/source-2.jpg"},
		},
		Variants: []canonical.Variant{
			{
				SKU: "SKU-1",
				Attributes: map[string]canonical.Attribute{
					"color": {Value: "Black"},
				},
				Price: &canonical.PriceInfo{
					Currency:  "USD",
					Amount:    29.99,
					CostPrice: 10.50,
				},
				Stock:     18,
				Barcode:   "1234567890",
				IsDefault: true,
			},
		},
	}

	imageResult := &productimage.ImageProcessResult{
		MainImage: &productimage.ImageAsset{
			URL: "https://cdn.example.com/main.jpg",
			Metadata: map[string]string{
				"scene_defaults_source": "platform_category",
				"scene_category":        "jewelry",
				"scene_style":           "studio",
				"background_tone":       "cool",
				"composition":           "close_up",
				"props_level":           "none",
				"audience_hint":         "premium",
				"prompt_key":            "productimage.scene.jewelry",
			},
		},
		WhiteBgImage: &productimage.ImageAsset{URL: "https://cdn.example.com/white.jpg"},
		GalleryImages: []productimage.ImageAsset{
			{URL: "https://cdn.example.com/gallery-1.jpg"},
		},
	}

	result := NewAssembler(stubAmazonDraftBuilder{}).Assemble(task, canonical, imageResult)

	if result.Amazon == nil || result.Amazon.Draft == nil {
		t.Fatal("expected amazon package")
	}
	if result.Shein == nil || result.Temu == nil || result.Walmart == nil {
		t.Fatal("expected shein/temu/walmart packages")
	}
	if result.Shein.BrandName != "DemoBrand" {
		t.Fatalf("shein brand = %q, want %q", result.Shein.BrandName, "DemoBrand")
	}
	if result.Shein.RequestDraft == nil {
		t.Fatal("expected shein request draft")
	}
	if result.Shein.RequestDraft.SpuName == "" {
		t.Fatal("expected shein request draft spu_name")
	}
	if len(result.Shein.RequestDraft.MultiLanguageDescList) == 0 {
		t.Fatal("expected shein request draft descriptions")
	}
	if len(result.Shein.SiteList) == 0 || result.Shein.SiteList[0].MainSite != "US" {
		t.Fatalf("shein site list = %#v, want US", result.Shein.SiteList)
	}
	if len(result.Shein.ProductAttributes) == 0 {
		t.Fatal("expected shein product attributes")
	}
	if len(result.Shein.RequestDraft.SKCList) != 1 {
		t.Fatalf("shein request skc count = %d, want 1", len(result.Shein.RequestDraft.SKCList))
	}
	if len(result.Shein.RequestDraft.SKCList[0].SKUList) != 1 {
		t.Fatalf("shein request sku count = %d, want 1", len(result.Shein.RequestDraft.SKCList[0].SKUList))
	}
	if result.Shein.RequestDraft.SKCList[0].SKUList[0].BasePrice == "" {
		t.Fatal("expected shein request sku base price")
	}
	if result.Shein.RequestDraft.SKCList[0].SKUList[0].WeightUnit == "" {
		t.Fatal("expected shein request sku weight unit")
	}
	if result.Shein.PreviewProduct == nil {
		t.Fatal("expected shein preview product")
	}
	if result.Shein.PreviewProduct.SPUName != result.Shein.RequestDraft.SpuName {
		t.Fatalf("shein preview spu_name = %q, want %q", result.Shein.PreviewProduct.SPUName, result.Shein.RequestDraft.SpuName)
	}
	if len(result.Shein.PreviewProduct.SiteList) != 1 || result.Shein.PreviewProduct.SiteList[0].MainSite != "US" {
		t.Fatalf("shein preview site list = %#v, want US", result.Shein.PreviewProduct.SiteList)
	}
	if len(result.Shein.PreviewProduct.SKCList) != 1 {
		t.Fatalf("shein preview skc count = %d, want 1", len(result.Shein.PreviewProduct.SKCList))
	}
	if len(result.Shein.PreviewProduct.SKCList[0].SKUS) != 1 {
		t.Fatalf("shein preview sku count = %d, want 1", len(result.Shein.PreviewProduct.SKCList[0].SKUS))
	}
	if len(result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList) != 1 {
		t.Fatalf("shein preview price count = %d, want 1", len(result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList))
	}
	if result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice != 29.99 {
		t.Fatalf("shein preview base price = %v, want 29.99", result.Shein.PreviewProduct.SKCList[0].SKUS[0].PriceInfoList[0].BasePrice)
	}
	if result.Shein.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU != "SKU-1" {
		t.Fatalf("shein preview supplier sku = %q, want %q", result.Shein.PreviewProduct.SKCList[0].SKUS[0].SupplierSKU, "SKU-1")
	}
	if result.Temu.Images == nil || result.Temu.Images.MainImage != "https://cdn.example.com/main.jpg" {
		t.Fatalf("temu main image not populated from image assets")
	}
	if result.Temu.BatchSkuInfo == nil || result.Temu.BatchSkuInfo.OutSkuSN != "SKU-1" {
		t.Fatalf("temu batch sku info = %+v, want out sku sn", result.Temu.BatchSkuInfo)
	}
	if result.Walmart.ProductType != "Headphones" {
		t.Fatalf("walmart product type = %q, want %q", result.Walmart.ProductType, "Headphones")
	}
	if result.ImageAssets == nil || result.ImageAssets.MainImage == nil {
		t.Fatalf("listingkit image assets = %+v", result.ImageAssets)
	}
	if result.ImageAssets.MainImage.Metadata["scene_defaults_source"] != "platform_category" ||
		result.ImageAssets.MainImage.Metadata["prompt_key"] != "productimage.scene.jewelry" {
		t.Fatalf("listingkit image metadata = %+v", result.ImageAssets.MainImage.Metadata)
	}
	if got := len(result.Shein.SkcList); got != 1 {
		t.Fatalf("shein skc count = %d, want 1", got)
	}
	if got := len(result.Shein.SkcList[0].SKUs); got != 1 {
		t.Fatalf("shein sku count = %d, want 1", got)
	}
	if result.Summary == nil || result.Summary.VariantCount != 1 {
		t.Fatalf("summary variant_count = %+v, want 1", result.Summary)
	}
}

func TestAssemblerResolvesSheinCategoryIntoPreviewProduct(t *testing.T) {
	t.Parallel()

	levelFourID := 4004
	levelFourName := "Wireless Earbuds"
	assembler := NewAssemblerWithConfig(AssemblerConfig{
		AmazonBuilder: stubAmazonDraftBuilder{},
		SheinCategoryResolver: sheinpub.NewCategoryResolver(stubSheinCategoryAPI{
			info: &sheincategory.CategoryInfo{
				ProductTypeID:          9001,
				LevelOneCategoryID:     1001,
				LevelOneCategoryName:   "Electronics",
				LevelTwoCategoryID:     2002,
				LevelTwoCategoryName:   "Audio",
				LevelThreeCategoryID:   3003,
				LevelThreeCategoryName: "Headphones",
				LevelFourCategoryID:    &levelFourID,
				LevelFourCategoryName:  &levelFourName,
			},
		}),
		SheinAttributeResolver: sheinpub.NewAttributeResolver(stubSheinAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{
				Data: []sheinattribute.AttributeTemplate{{
					AttributeInfos: []sheinattribute.AttributeInfo{
						{
							AttributeID:     501,
							AttributeName:   "颜色",
							AttributeNameEn: "Color",
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 90001, AttributeValue: "黑色", AttributeValueEn: "Black"},
							},
						},
						{
							AttributeID:     502,
							AttributeName:   "尺寸",
							AttributeNameEn: "Size",
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 90002, AttributeValue: "中码", AttributeValueEn: "M"},
							},
						},
					},
				}},
			},
		}, nil),
		SheinSaleAttributeResolver: sheinpub.NewSaleAttributeResolver(stubSheinAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{
				Data: []sheinattribute.AttributeTemplate{{
					AttributeInfos: []sheinattribute.AttributeInfo{
						{
							AttributeID:     501,
							AttributeName:   "颜色",
							AttributeNameEn: "Color",
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 90001, AttributeValue: "黑色", AttributeValueEn: "Black"},
							},
						},
						{
							AttributeID:     502,
							AttributeName:   "尺寸",
							AttributeNameEn: "Size",
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 90002, AttributeValue: "中码", AttributeValueEn: "M"},
							},
						},
					},
				}},
			},
		}, nil),
	})

	task := &Task{
		ID: "task-2",
		Request: &GenerateRequest{
			Text:               "test product",
			Platforms:          []string{"shein"},
			Country:            "US",
			Language:           "en_US",
			TargetCategoryHint: "4004",
		},
	}

	canonical := &canonical.Product{
		Title:        "Wireless Earbuds",
		CategoryPath: []string{"Electronics", "Headphones"},
		Attributes: map[string]canonical.Attribute{
			"color": {Value: "Black"},
			"size":  {Value: "M"},
		},
		Images: []canonical.Image{
			{URL: "https://example.com/source-main.jpg"},
		},
		Variants: []canonical.Variant{
			{
				SKU: "SKU-1",
				Attributes: map[string]canonical.Attribute{
					"color": {Value: "Black"},
					"size":  {Value: "M"},
				},
				Price: &canonical.PriceInfo{
					Currency: "USD",
					Amount:   29.99,
				},
				Stock: 10,
			},
		},
	}

	result := assembler.Assemble(task, canonical, nil)

	if result.Shein == nil || result.Shein.CategoryResolution == nil {
		t.Fatal("expected shein category resolution")
	}
	if result.Shein.CategoryResolution.Status != "resolved" {
		t.Fatalf("shein category resolution status = %q, want %q", result.Shein.CategoryResolution.Status, "resolved")
	}
	if result.Shein.CategoryID != 4004 {
		t.Fatalf("shein category id = %d, want 4004", result.Shein.CategoryID)
	}
	if result.Shein.TopCategoryID != 1001 {
		t.Fatalf("shein top category id = %d, want 1001", result.Shein.TopCategoryID)
	}
	if result.Shein.ProductTypeID == nil || *result.Shein.ProductTypeID != 9001 {
		t.Fatalf("shein product type id = %#v, want 9001", result.Shein.ProductTypeID)
	}
	if len(result.Shein.CategoryIDList) != 4 {
		t.Fatalf("shein category id list = %#v, want 4 levels", result.Shein.CategoryIDList)
	}
	if result.Shein.PreviewProduct == nil {
		t.Fatal("expected shein preview product")
	}
	if result.Shein.AttributeResolution == nil {
		t.Fatal("expected shein attribute resolution")
	}
	if result.Shein.AttributeResolution.ResolvedCount != 2 {
		t.Fatalf("shein resolved attribute count = %d, want 2", result.Shein.AttributeResolution.ResolvedCount)
	}
	if len(result.Shein.ResolvedAttributes) != 2 {
		t.Fatalf("shein resolved attributes = %#v, want 2", result.Shein.ResolvedAttributes)
	}
	if !containsResolvedAttribute(result.Shein.ResolvedAttributes, 501, 90001) {
		t.Fatalf("shein resolved attributes = %#v, want color mapping", result.Shein.ResolvedAttributes)
	}
	if result.Shein.PreviewProduct.CategoryID != 4004 {
		t.Fatalf("shein preview category id = %d, want 4004", result.Shein.PreviewProduct.CategoryID)
	}
	if result.Shein.SaleAttributeResolution == nil {
		t.Fatal("expected shein sale attribute resolution")
	}
	if result.Shein.Inspection == nil {
		t.Fatal("expected shein inspection")
	}
	if len(result.Shein.Inspection.Sections) != 3 {
		t.Fatalf("shein inspection sections = %#v, want 3", result.Shein.Inspection.Sections)
	}
	if len(result.Shein.SaleAttributeResolution.Candidates) < 2 {
		t.Fatalf("shein sale attribute candidates = %#v, want at least 2", result.Shein.SaleAttributeResolution.Candidates)
	}
	if len(result.Shein.SaleAttributeResolution.SelectionSummary) == 0 {
		t.Fatal("expected shein sale attribute selection summary")
	}
	if result.Shein.SaleAttributeResolution.PrimaryAttributeID != 501 {
		t.Fatalf("shein primary sale attribute id = %d, want 501", result.Shein.SaleAttributeResolution.PrimaryAttributeID)
	}
	if result.Shein.SaleAttributeResolution.SecondaryAttributeID != 502 {
		t.Fatalf("shein secondary sale attribute id = %d, want 502", result.Shein.SaleAttributeResolution.SecondaryAttributeID)
	}
	if result.Shein.PreviewProduct.ProductTypeID == nil || *result.Shein.PreviewProduct.ProductTypeID != 9001 {
		t.Fatalf("shein preview product type id = %#v, want 9001", result.Shein.PreviewProduct.ProductTypeID)
	}
	if got := result.Shein.PreviewProduct.CategoryIDList; len(got) != 4 || got[0] != 1001 || got[3] != 4004 {
		t.Fatalf("shein preview category id list = %#v, want [1001 ... 4004]", got)
	}
	if len(result.Shein.PreviewProduct.ProductAttributeList) != 2 {
		t.Fatalf("shein preview product attributes = %#v, want 2", result.Shein.PreviewProduct.ProductAttributeList)
	}
	if result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID != 501 {
		t.Fatalf("shein preview skc sale attribute id = %d, want 501", result.Shein.PreviewProduct.SKCList[0].SaleAttribute.AttributeID)
	}
	if len(result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList) != 1 {
		t.Fatalf("shein preview sku sale attributes = %#v, want 1", result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList)
	}
	if result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList[0].AttributeID != 502 {
		t.Fatalf("shein preview sku sale attribute id = %d, want 502", result.Shein.PreviewProduct.SKCList[0].SKUS[0].SaleAttributeList[0].AttributeID)
	}
}

func TestBuildPlatformImagesFallsBackToCanonicalImages(t *testing.T) {
	t.Parallel()

	canonical := &canonical.Product{
		Images: []canonical.Image{
			{URL: "https://example.com/1.jpg"},
			{URL: "https://example.com/2.jpg"},
		},
	}

	images := buildPlatformImages(canonical, nil)
	if images == nil {
		t.Fatal("expected images")
	}
	if images.MainImage != "https://example.com/1.jpg" {
		t.Fatalf("main image = %q, want canonical first image", images.MainImage)
	}
	if len(images.Gallery) != 1 || images.Gallery[0] != "https://example.com/2.jpg" {
		t.Fatalf("gallery = %#v, want second canonical image", images.Gallery)
	}
}

func TestManagedSheinCategoryResolverFallsBackWithoutStoreID(t *testing.T) {
	t.Parallel()

	resolver := sheinmanaged.NewCategoryResolver(nil)
	req := &GenerateRequest{
		Text:      "wireless earbuds",
		Country:   "US",
		Language:  "en_US",
		Platforms: []string{"shein"},
	}
	canonical := &canonical.Product{
		Title:        "Wireless Earbuds",
		CategoryPath: []string{"Electronics", "Headphones"},
	}
	pkg := &SheinPackage{
		CategoryName: "Headphones",
		CategoryPath: []string{"Electronics", "Headphones"},
	}

	resolution := resolver.Resolve(buildSheinPublishRequest(req), canonical, pkg)
	if resolution == nil {
		t.Fatal("expected resolution")
	}
	if resolution.Status != "partial" {
		t.Fatalf("resolution status = %q, want %q", resolution.Status, "partial")
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected review notes when shein_store_id is missing")
	}
}

func TestManagedSheinAttributeResolverFallsBackWithoutStoreID(t *testing.T) {
	t.Parallel()

	resolver := sheinmanaged.NewAttributeResolver(nil, nil)
	req := &GenerateRequest{
		Text:      "wireless earbuds",
		Country:   "US",
		Language:  "en_US",
		Platforms: []string{"shein"},
	}
	canonical := &canonical.Product{
		Title: "Wireless Earbuds",
	}
	pkg := &SheinPackage{
		CategoryID: 4004,
		ProductAttributes: []PlatformAttribute{
			{Name: "color", Value: "Black"},
		},
	}

	resolution := resolver.Resolve(buildSheinPublishRequest(req), canonical, pkg)
	if resolution == nil {
		t.Fatal("expected resolution")
	}
	if resolution.Status != "partial" {
		t.Fatalf("resolution status = %q, want %q", resolution.Status, "partial")
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected review notes when shein_store_id is missing")
	}
}

func TestManagedSheinSaleAttributeResolverFallsBackWithoutStoreID(t *testing.T) {
	t.Parallel()

	resolver := sheinmanaged.NewSaleAttributeResolver(nil, nil)
	req := &GenerateRequest{
		Text:      "wireless earbuds",
		Country:   "US",
		Language:  "en_US",
		Platforms: []string{"shein"},
	}
	canonical := &canonical.Product{
		Title: "Wireless Earbuds",
	}
	pkg := &SheinPackage{
		CategoryID: 4004,
		SkcList: []SheinSKCPackage{{
			Attributes: map[string]string{"color": "Black"},
			SKUs: []PlatformVariant{{
				Attributes: map[string]string{"size": "M"},
			}},
		}},
	}

	resolution := resolver.Resolve(buildSheinPublishRequest(req), canonical, pkg)
	if resolution == nil {
		t.Fatal("expected resolution")
	}
	if resolution.Status != "partial" {
		t.Fatalf("resolution status = %q, want %q", resolution.Status, "partial")
	}
	if len(resolution.ReviewNotes) == 0 {
		t.Fatal("expected review notes when shein_store_id is missing")
	}
}

func TestBuildSheinPublishRequestForTaskIncludesTaskIdentity(t *testing.T) {
	t.Parallel()

	task := &Task{TenantID: "tenant-1", UserID: "user-1"}
	req := &GenerateRequest{
		Country:            "US",
		Language:           "en_US",
		Text:               "wireless earbuds",
		BrandHint:          "Acme",
		TargetCategoryHint: "4004",
		SheinStoreID:       42,
	}

	built := buildSheinPublishRequestForTask(task, req)
	if built == nil {
		t.Fatal("expected build request")
	}
	if built.SheinStoreID != 42 {
		t.Fatalf("shein_store_id = %d, want 42", built.SheinStoreID)
	}
	if got := TenantIDFromContext(built.Context); got != "tenant-1" {
		t.Fatalf("tenant id in context = %q, want %q", got, "tenant-1")
	}
	if identity := openaiclient.IdentityFromContext(built.Context); identity.UserID != "user-1" || identity.TenantID != "tenant-1" {
		t.Fatalf("identity from context = %#v, want tenant-1/user-1", identity)
	}
}

func TestBuildSummaryIncludesCanonicalAndImageReviewState(t *testing.T) {
	t.Parallel()

	summary := buildSummary(&Task{
		Request: &GenerateRequest{
			Text:      "wireless earbuds",
			ImageURLs: []string{"https://example.com/1.jpg", "https://example.com/2.jpg"},
		},
	}, &canonical.Product{
		NeedsReview: true,
		Variants: []canonical.Variant{
			{SKU: "SKU-1"},
			{SKU: "SKU-2"},
		},
	}, &productimage.ImageProcessResult{
		Review: &productimage.ReviewDecision{
			NeedsReview: true,
			Reasons:     []string{"image needs manual review"},
		},
	})
	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.SourceType != "images_and_text" || summary.ImageCount != 2 || summary.VariantCount != 2 {
		t.Fatalf("summary core fields = %+v", summary)
	}
	if !summary.NeedsReview {
		t.Fatalf("summary needs_review = false, want true: %+v", summary)
	}
	if len(summary.Warnings) != 1 || summary.Warnings[0] != "image needs manual review" {
		t.Fatalf("summary warnings = %#v, want image review reason", summary.Warnings)
	}
}

func containsResolvedAttribute(items []SheinResolvedAttribute, attributeID int, valueID int) bool {
	for _, item := range items {
		if item.AttributeID == attributeID && item.AttributeValueID != nil && *item.AttributeValueID == valueID {
			return true
		}
	}
	return false
}
