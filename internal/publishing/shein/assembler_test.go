package shein

import (
	"strings"
	"testing"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

type stubAttributeAPI struct {
	templates      *sheinattribute.AttributeTemplateInfo
	validateCustom func(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error)
	addCustom      func(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error)
}

type assemblerStubCategoryAPI struct {
	info *sheincategory.CategoryInfo
}

type assemblerStubCategoryResolver struct {
	resolution *CategoryResolution
}

type assemblerStubSaleAttributeResolver struct {
	resolution *SaleAttributeResolution
}

func (s assemblerStubCategoryResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *CategoryResolution {
	if s.resolution == nil {
		return &CategoryResolution{}
	}
	cloned := *s.resolution
	if len(s.resolution.MatchedPath) > 0 {
		cloned.MatchedPath = append([]string(nil), s.resolution.MatchedPath...)
	}
	if len(s.resolution.CategoryIDList) > 0 {
		cloned.CategoryIDList = append([]int(nil), s.resolution.CategoryIDList...)
	}
	return &cloned
}

func (s assemblerStubSaleAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	if s.resolution == nil {
		return &SaleAttributeResolution{}
	}
	cloned := *s.resolution
	cloned.ReviewNotes = append([]string(nil), s.resolution.ReviewNotes...)
	return &cloned
}

func (s assemblerStubCategoryAPI) GetCategory(categoryID int) (*sheincategory.CategoryInfo, error) {
	if s.info == nil {
		return nil, nil
	}
	info := *s.info
	info.CategoryID = categoryID
	return &info, nil
}

func (assemblerStubCategoryAPI) GetCategoryTree() (*sheincategory.CategoryTreeResponse, error) {
	return nil, nil
}

func (assemblerStubCategoryAPI) SuggestCategoryByText(string) (*sheincategory.SuggestCategoryResponse, error) {
	return nil, nil
}

func (s stubAttributeAPI) GetAttributeTemplates(categoryID int) (*sheinattribute.AttributeTemplateInfo, error) {
	return s.templates, nil
}

func (s stubAttributeAPI) ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error) {
	if s.validateCustom != nil {
		return s.validateCustom(attributeID, attributeValue, categoryID, spuName)
	}
	return nil, nil
}

func (s stubAttributeAPI) AddCustomAttributeValue(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error) {
	if s.addCustom != nil {
		return s.addCustom(req)
	}
	return nil, nil
}

func TestBuildRequestSKCsGroupsVariantsByColor(t *testing.T) {
	canonical := testCanonicalProduct()
	variants := common.BuildVariants(canonical)
	groups := buildVariantGroups("Running Shoes", variants, &common.ImageSet{MainImage: "main.jpg"}, &SaleAttributeResolution{
		PrimarySourceDimension: "color",
	})

	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}
	if groups[0].skcName != "Running Shoes - Red" {
		t.Fatalf("first group skc name = %q, want Running Shoes - Red", groups[0].skcName)
	}
	if len(groups[0].skus) != 2 {
		t.Fatalf("first group sku count = %d, want 2", len(groups[0].skus))
	}
	if groups[0].attributes["color"] != "Red" {
		t.Fatalf("first group color = %q, want Red", groups[0].attributes["color"])
	}
	if _, ok := groups[0].attributes["size"]; ok {
		t.Fatalf("expected varying size to be excluded from group-level attributes")
	}

	requestSKCs := buildRequestSKCs(groups, &common.ImageSet{MainImage: "main.jpg"}, common.DefaultSites("US"), canonical, PricingPolicy{})
	if len(requestSKCs) != 2 {
		t.Fatalf("request skc count = %d, want 2", len(requestSKCs))
	}
	if len(requestSKCs[0].SKUList) != 2 {
		t.Fatalf("first request skc sku count = %d, want 2", len(requestSKCs[0].SKUList))
	}
}

func TestSaleAttributeResolutionAppliesAssignmentsAcrossGroupedSKCs(t *testing.T) {
	canonical := testCanonicalProduct()
	variants := common.BuildVariants(canonical)
	images := &common.ImageSet{MainImage: "main.jpg"}
	groups := buildVariantGroups("Running Shoes", variants, images, &SaleAttributeResolution{
		PrimarySourceDimension: "color",
	})
	pkg := &Package{
		CategoryID: 100,
		SkcList:    buildSKCs(groups),
		RequestDraft: &RequestDraft{
			SKCList: buildRequestSKCs(groups, images, common.DefaultSites("US"), canonical, PricingPolicy{}),
		},
	}

	resolver := NewSaleAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       501,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "红色", AttributeValueEn: "Red"},
							{AttributeValueID: 12, AttributeValue: "蓝色", AttributeValueEn: "Blue"},
						},
					},
					{
						AttributeID:       502,
						AttributeName:     "尺码",
						AttributeNameEn:   "Size",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 21, AttributeValue: "42", AttributeValueEn: "42"},
							{AttributeValueID: 22, AttributeValue: "43", AttributeValueEn: "43"},
						},
					},
				},
			}},
		},
	}, nil)

	resolution := resolver.Resolve(&BuildRequest{}, canonical, pkg)
	ApplySaleAttributeResolution(pkg, resolution)

	if pkg.RequestDraft.SKCList[0].SaleAttribute == nil || pkg.RequestDraft.SKCList[0].SaleAttribute.AttributeValueID == nil || *pkg.RequestDraft.SKCList[0].SaleAttribute.AttributeValueID != 11 {
		t.Fatalf("first skc sale attribute = %+v, want red value id 11", pkg.RequestDraft.SKCList[0].SaleAttribute)
	}
	if pkg.RequestDraft.SKCList[1].SaleAttribute == nil || pkg.RequestDraft.SKCList[1].SaleAttribute.AttributeValueID == nil || *pkg.RequestDraft.SKCList[1].SaleAttribute.AttributeValueID != 12 {
		t.Fatalf("second skc sale attribute = %+v, want blue value id 12", pkg.RequestDraft.SKCList[1].SaleAttribute)
	}
	if len(pkg.RequestDraft.SKCList[0].SKUList[0].SaleAttributes) != 1 || pkg.RequestDraft.SKCList[0].SKUList[0].SaleAttributes[0].AttributeValueID == nil || *pkg.RequestDraft.SKCList[0].SKUList[0].SaleAttributes[0].AttributeValueID != 21 {
		t.Fatalf("first sku sale attributes = %+v, want size value id 21", pkg.RequestDraft.SKCList[0].SKUList[0].SaleAttributes)
	}
	if len(pkg.RequestDraft.SKCList[0].SKUList[1].SaleAttributes) != 1 || pkg.RequestDraft.SKCList[0].SKUList[1].SaleAttributes[0].AttributeValueID == nil || *pkg.RequestDraft.SKCList[0].SKUList[1].SaleAttributes[0].AttributeValueID != 22 {
		t.Fatalf("second sku sale attributes = %+v, want size value id 22", pkg.RequestDraft.SKCList[0].SKUList[1].SaleAttributes)
	}
	if len(pkg.RequestDraft.SKCList[1].SKUList[0].SaleAttributes) != 1 || pkg.RequestDraft.SKCList[1].SKUList[0].SaleAttributes[0].AttributeValueID == nil || *pkg.RequestDraft.SKCList[1].SKUList[0].SaleAttributes[0].AttributeValueID != 21 {
		t.Fatalf("third sku sale attributes = %+v, want size value id 21", pkg.RequestDraft.SKCList[1].SKUList[0].SaleAttributes)
	}
}

func TestAssemblerBuildCreatesDefaultSKCWhenCanonicalVariantsMissing(t *testing.T) {
	assembler := NewAssembler(AssemblerConfig{})
	canonical := &canonical.Product{
		Title:       "Minimal Product",
		Description: "fallback variant",
		Attributes: map[string]canonical.Attribute{
			"color": {Value: "Black"},
			"size":  {Value: "One Size"},
			"price": {Value: "19.9"},
		},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", TargetCategoryHint: "4004"}, canonical, nil)
	if pkg == nil || pkg.RequestDraft == nil {
		t.Fatalf("expected package with request draft")
	}
	if len(pkg.RequestDraft.SKCList) != 1 {
		t.Fatalf("request draft skc count = %d, want 1", len(pkg.RequestDraft.SKCList))
	}
	if len(pkg.RequestDraft.SKCList[0].SKUList) != 1 {
		t.Fatalf("request draft sku count = %d, want 1", len(pkg.RequestDraft.SKCList[0].SKUList))
	}
	if pkg.RequestDraft.SKCList[0].SKUList[0].SupplierSKU != "DEFAULT-001" {
		t.Fatalf("default supplier sku = %q, want DEFAULT-001", pkg.RequestDraft.SKCList[0].SKUList[0].SupplierSKU)
	}
	if pkg.PreviewProduct == nil || len(pkg.PreviewProduct.SKCList) != 1 {
		t.Fatalf("preview product skc count = %d, want 1", len(pkg.PreviewProduct.SKCList))
	}
}

func TestAssemblerBuildMarks1688LLMOnlyFactsForReview(t *testing.T) {
	assembler := NewAssembler(AssemblerConfig{})
	canonical := &canonical.Product{
		Title:         "1688 Product",
		Description:   "Scraped description",
		SellingPoints: []string{"LLM-only waterproof claim"},
		FieldTraces: map[string]canonical.FieldTrace{
			"title": {
				Sources: []canonical.Source{
					{Type: canonical.SourceProductURL, Detail: "https://detail.1688.com/offer/123.html"},
					{Type: canonical.SourceScrapedData, Detail: "scraped title"},
				},
			},
			"selling_points": {
				Sources: []canonical.Source{
					{Type: canonical.SourceProductURL, Detail: "https://detail.1688.com/offer/123.html"},
					{Type: canonical.SourceScrapedData, Detail: "normalized from product page: https://detail.1688.com/offer/123.html"},
					{Type: canonical.SourceLLM, Detail: "LLM-generated product normalization"},
				},
				NeedsReview: true,
			},
		},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en"}, canonical, nil)

	if pkg == nil || pkg.Metadata == nil {
		t.Fatal("expected package metadata")
	}
	if pkg.Metadata["source_platform"] != "1688" {
		t.Fatalf("source_platform = %q, want 1688", pkg.Metadata["source_platform"])
	}
	if pkg.Metadata["source_fact_review_required"] != "true" {
		t.Fatalf("source_fact_review_required = %q, want true", pkg.Metadata["source_fact_review_required"])
	}
	if pkg.Metadata["source_fact_review_fields"] != "selling_points" {
		t.Fatalf("source_fact_review_fields = %q, want selling_points", pkg.Metadata["source_fact_review_fields"])
	}
}

func TestAssemblerBuildAppliesPricingPolicyToRequestDraft(t *testing.T) {
	assembler := NewAssembler(AssemblerConfig{
		PricingPolicy: PricingPolicy{
			Enabled:      true,
			ShippingCost: 2,
			MarkupRate:   0.25,
			RoundTo:      0.01,
		},
	})
	canonical := &canonical.Product{
		Title: "Priced Product",
		Variants: []canonical.Variant{{
			SKU: "SKU-1",
			Price: &canonical.PriceInfo{
				Currency:  "USD",
				Amount:    12,
				CostPrice: 8,
			},
			Stock:     5,
			IsDefault: true,
		}},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en"}, canonical, nil)

	if len(pkg.RequestDraft.SKCList) != 1 || len(pkg.RequestDraft.SKCList[0].SKUList) != 1 {
		t.Fatalf("request draft skus = %+v", pkg.RequestDraft.SKCList)
	}
	sku := pkg.RequestDraft.SKCList[0].SKUList[0]
	if sku.CostPrice != "8" {
		t.Fatalf("cost price = %q, want 8", sku.CostPrice)
	}
	if sku.BasePrice != "12.5" {
		t.Fatalf("base price = %q, want 12.5", sku.BasePrice)
	}
	if len(sku.SitePriceList) != 1 || sku.SitePriceList[0].BasePrice != "12.5" {
		t.Fatalf("site price list = %+v, want 12.5", sku.SitePriceList)
	}
}

func TestBuildRequestSKCsPreferVariantSpecificDimensions(t *testing.T) {
	canonical := &canonical.Product{
		Title: "Floor Mat",
		Specifications: &canonical.ProductSpecs{
			Dimensions: &canonical.Dimensions{Length: 99, Width: 88, Height: 7, Unit: "cm"},
		},
		Variants: []canonical.Variant{
			{
				SKU:        "SKU-40",
				Attributes: map[string]canonical.Attribute{"Color": {Value: "White"}, "Size": {Value: "40x60cm"}},
				Dimensions: &canonical.Dimensions{Length: 40, Width: 30, Height: 2, Unit: "cm"},
				Stock:      5,
				IsDefault:  true,
			},
			{
				SKU:        "SKU-50",
				Attributes: map[string]canonical.Attribute{"Color": {Value: "White"}, "Size": {Value: "50x80cm"}},
				Dimensions: &canonical.Dimensions{Length: 50, Width: 40, Height: 3, Unit: "cm"},
				Stock:      5,
			},
		},
	}

	variants := common.BuildVariants(canonical)
	groups := buildVariantGroups("Floor Mat", variants, &common.ImageSet{MainImage: "main.jpg"}, &SaleAttributeResolution{
		PrimarySourceDimension:   "Color",
		SecondarySourceDimension: "Size",
	})
	requestSKCs := buildRequestSKCs(groups, &common.ImageSet{MainImage: "main.jpg"}, common.DefaultSites("US"), canonical, PricingPolicy{})
	if len(requestSKCs) != 1 || len(requestSKCs[0].SKUList) != 2 {
		t.Fatalf("request skcs = %+v", requestSKCs)
	}
	if requestSKCs[0].SKUList[0].Length != "40" || requestSKCs[0].SKUList[0].Width != "30" || requestSKCs[0].SKUList[0].Height != "2" {
		t.Fatalf("first sku dimensions = %+v", requestSKCs[0].SKUList[0])
	}
	if requestSKCs[0].SKUList[1].Length != "50" || requestSKCs[0].SKUList[1].Width != "40" || requestSKCs[0].SKUList[1].Height != "3" {
		t.Fatalf("second sku dimensions = %+v", requestSKCs[0].SKUList[1])
	}
}

func TestAssemblerBuildCreatesGroupedSKCsWhenSaleAttributeResolverMapsSourceDimensions(t *testing.T) {
	assembler := NewAssembler(AssemblerConfig{
		CategoryResolver: NewCategoryResolver(assemblerStubCategoryAPI{
			info: &sheincategory.CategoryInfo{
				LevelOneCategoryID:   1001,
				LevelOneCategoryName: "Shoes",
			},
		}),
		SaleAttributeResolver: NewSaleAttributeResolver(stubAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{
				Data: []sheinattribute.AttributeTemplate{{
					AttributeInfos: []sheinattribute.AttributeInfo{
						{
							AttributeID:       501,
							AttributeName:     "颜色",
							AttributeNameEn:   "Color",
							AttributeType:     1,
							SKCScope:          boolPointer(true),
							AttributeInputNum: 1,
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 11, AttributeValue: "红色", AttributeValueEn: "Red"},
								{AttributeValueID: 12, AttributeValue: "蓝色", AttributeValueEn: "Blue"},
							},
						},
						{
							AttributeID:       502,
							AttributeName:     "尺码",
							AttributeNameEn:   "Size",
							AttributeInputNum: 1,
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 21, AttributeValue: "42", AttributeValueEn: "42"},
								{AttributeValueID: 22, AttributeValue: "43", AttributeValueEn: "43"},
							},
						},
					},
				}},
			},
		}, nil),
	})
	canonical := &canonical.Product{
		Title:       "Fallback Matrix Product",
		Description: "fallback matrix",
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "color", Values: []string{"Red", "Blue"}},
			{Name: "size", Values: []string{"42", "43"}},
		},
		Attributes: map[string]canonical.Attribute{
			"color": {Value: "Red, Blue"},
			"size":  {Value: "42/43"},
		},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", TargetCategoryHint: "4004"}, canonical, nil)
	if pkg == nil || pkg.RequestDraft == nil {
		t.Fatalf("expected package with request draft")
	}
	if len(pkg.RequestDraft.SKCList) != 2 {
		t.Fatalf("request draft skc count = %d, want 2; resolution=%#v", len(pkg.RequestDraft.SKCList), pkg.SaleAttributeResolution)
	}
	if len(pkg.RequestDraft.SKCList[0].SKUList) != 2 {
		t.Fatalf("first skc sku count = %d, want 2", len(pkg.RequestDraft.SKCList[0].SKUList))
	}
	if pkg.RequestDraft.SKCList[0].SkcName != "Fallback Matrix Product - Red" || pkg.RequestDraft.SKCList[1].SkcName != "Fallback Matrix Product - Blue" {
		t.Fatalf("skc names = %q, %q; want Fallback Matrix Product - Red/Fallback Matrix Product - Blue", pkg.RequestDraft.SKCList[0].SkcName, pkg.RequestDraft.SKCList[1].SkcName)
	}
}

func TestAssemblerBuildDoesNotPropagatePromptTextIntoSKCNames(t *testing.T) {
	assembler := NewAssembler(AssemblerConfig{
		SaleAttributeResolver: NewSaleAttributeResolver(stubAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{
				Data: []sheinattribute.AttributeTemplate{{
					AttributeInfos: []sheinattribute.AttributeInfo{{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "Black", AttributeValueEn: "Black"},
						},
					}},
				}},
			},
		}, nil),
	})
	canonical := &canonical.Product{
		Title: "Flannel non-slip floor mat",
		Attributes: map[string]canonical.Attribute{
			"product_english_name": {Value: "Flannel non-slip floor mat - Please design an image that can be printed on my non-slip floor mat. The image should include suitable English text and graphics, and the graphics and text should have a 3D visual effect. Please ensure it does not infringe on copyright. 3000 pixels * 2"},
		},
		VariantDimensions: []canonical.ScrapedVariantDimension{{Name: "color", Values: []string{"Black"}}},
		Variants: []canonical.Variant{
			{SKU: "SKU-BLACK", Attributes: map[string]canonical.Attribute{"color": {Value: "Black"}}},
		},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en"}, canonical, nil)
	if pkg.ProductNameEn != "Flannel non-slip floor mat" {
		t.Fatalf("product title = %q, want sanitized title", pkg.ProductNameEn)
	}
	if len(pkg.SkcList) != 1 {
		t.Fatalf("skc count = %d, want 1", len(pkg.SkcList))
	}
	if pkg.SkcList[0].SkcName != "Flannel non-slip floor mat - Black" {
		t.Fatalf("skc title = %q, want sanitized short title", pkg.SkcList[0].SkcName)
	}
	if pkg.TitleDiagnostics == nil || !pkg.TitleDiagnostics.PromptContaminated {
		t.Fatalf("title diagnostics = %+v, want contamination recorded", pkg.TitleDiagnostics)
	}
}

func TestBuildVariantGroupsUsesSanitizedSKCValueAssignmentsForPromptLikeGroupNames(t *testing.T) {
	sanitizedID := 1443
	rawPrompt := "帮我设计一个印在门帘上的图案，图案要有英文跟图案，元素多样，图片有3d视觉效果，摇滚风格，2277 × 4500px"
	variants := []common.Variant{
		{
			SKU: "MG8014104001-F0509EE2",
			Attributes: map[string]string{
				"ai_style": rawPrompt,
				"Size":     "90x180cm",
				"Color":    "white",
			},
		},
	}
	groups := buildVariantGroups("Door curtain", variants, &common.ImageSet{MainImage: "main.jpg"}, &SaleAttributeResolution{
		PrimarySourceDimension: "ai_style",
		skcValueAssignments: map[string]ResolvedSaleAttribute{
			normalizeText(rawPrompt): {
				Scope:            "skc",
				Name:             "Style Type",
				Value:            "rock style",
				AttributeID:      1001184,
				AttributeValueID: &sanitizedID,
				MatchedBy:        "attribute_value_comparable",
			},
		},
	})
	if len(groups) != 1 {
		t.Fatalf("group count = %d, want 1", len(groups))
	}
	if groups[0].skcName != "Door curtain - rock style" {
		t.Fatalf("skc title = %q, want sanitized sale attribute value", groups[0].skcName)
	}
}

func TestAssemblerBuildDoesNotTriggerRuleBasedCategoryReview(t *testing.T) {
	assembler := NewAssembler(AssemblerConfig{
		CategoryResolver: assemblerStubCategoryResolver{
			resolution: &CategoryResolution{
				Status:      "resolved",
				Source:      "target_category_hint",
				CategoryID:  12143,
				MatchedPath: []string{"家居&生活", "家庭用品", "鞋用品", "鞋配饰"},
			},
		},
		SaleAttributeResolver: NewSaleAttributeResolver(stubAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{
				Data: []sheinattribute.AttributeTemplate{{
					AttributeInfos: []sheinattribute.AttributeInfo{
						{
							AttributeID:       27,
							AttributeName:     "颜色",
							AttributeNameEn:   "Color",
							AttributeType:     1,
							SKCScope:          boolPointer(true),
							AttributeInputNum: 1,
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 11, AttributeValue: "黑色", AttributeValueEn: "Black"},
							},
						},
					},
				}},
			},
		}, nil),
	})

	canonical := &canonical.Product{
		Title:        "420ml stainless steel tumbler",
		Description:  "Vacuum insulated drinkware cup",
		CategoryPath: []string{"Drinkware", "Tumblers & Water Bottles"},
		Attributes: map[string]canonical.Attribute{
			"材质": {Value: "不锈钢"},
			"容量": {Value: "420ml"},
		},
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "颜色", Values: []string{"裸粉", "黑色"}},
		},
		Variants: []canonical.Variant{
			{SKU: "SKU-PINK", Attributes: map[string]canonical.Attribute{"颜色": {Value: "裸粉"}}},
			{SKU: "SKU-BLACK", Attributes: map[string]canonical.Attribute{"颜色": {Value: "黑色"}}},
		},
		Images: []canonical.Image{
			{URL: "main.jpg"},
		},
	}

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", SheinStoreID: 869, TargetCategoryHint: "12143"}, canonical, nil)
	if pkg.SaleAttributeResolution == nil {
		t.Fatal("expected sale attribute resolution")
	}
	if pkg.SaleAttributeResolution.RecommendCategoryReview {
		t.Fatalf("expected no rule-based category review: %+v", pkg.SaleAttributeResolution)
	}
	if pkg.CategoryResolution == nil {
		t.Fatal("expected category resolution")
	}
	if pkg.CategoryResolution.SuggestedCategory != nil {
		t.Fatalf("expected no suggested category without category review trigger, got %+v", pkg.CategoryResolution.SuggestedCategory)
	}
}

func TestAssemblerBuildDoesNotRequestAlternativeCategoryOnSaleReview(t *testing.T) {
	assembler := NewAssembler(AssemblerConfig{
		CategoryResolver: assemblerStubCategoryResolver{
			resolution: &CategoryResolution{
				Status:      "resolved",
				Source:      "target_category_hint",
				CategoryID:  12143,
				MatchedPath: []string{"家居&生活", "家庭用品", "鞋用品", "鞋配饰"},
			},
		},
		SaleAttributeResolver: assemblerStubSaleAttributeResolver{
			resolution: &SaleAttributeResolution{
				Status:                  "partial",
				CategoryID:              12143,
				RecommendCategoryReview: true,
				CategoryReviewReason:    "current category needs review",
			},
		},
	})

	product := testCanonicalProduct()
	product.Images = []canonical.Image{{URL: "main.jpg"}}
	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", SheinStoreID: 869, TargetCategoryHint: "12143"}, product, nil)
	if pkg.CategoryResolution == nil {
		t.Fatal("expected category resolution")
	}
	if pkg.CategoryResolution.SuggestedCategory != nil {
		t.Fatalf("expected no secondary category suggestion, got %+v", pkg.CategoryResolution.SuggestedCategory)
	}
	for _, note := range pkg.ReviewNotes {
		if strings.Contains(note, "可尝试候选类目") {
			t.Fatalf("expected no secondary category review note, got %v", pkg.ReviewNotes)
		}
	}
}

func TestAssemblerBuildReusesPublishedResolutionCacheOnSecondBuild(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	req := &BuildRequest{Country: "US", Language: "en", SheinStoreID: 42}
	product := &canonical.Product{
		Title:       "抱枕套 MG8014192",
		Description: "中文描述",
		CategoryPath: []string{
			"家居",
			"装饰",
			"抱枕套",
		},
		Attributes: map[string]canonical.Attribute{
			"material": {Value: "涤纶"},
			"color":    {Value: "White"},
		},
		Variants: []canonical.Variant{{
			SKU: "MG8014192",
			Attributes: map[string]canonical.Attribute{
				"Color": {Value: "White"},
				"Size":  {Value: "45x45cm"},
			},
			Dimensions: &canonical.Dimensions{Length: 45, Width: 45, Height: 1, Unit: "cm"},
			Stock:      5,
			IsDefault:  true,
		}},
		Images: []canonical.Image{{URL: "main.jpg"}},
	}
	categoryInner := &countingCategoryResolver{
		out: &CategoryResolution{
			Status:         "resolved",
			Source:         "test",
			CategoryID:     8218,
			CategoryIDList: []int{2030, 6012, 8218},
			MatchedPath:    []string{"Home", "Decor", "Cushion Covers"},
		},
	}
	valueID := 2001
	attributeInner := &countingAttributeResolver{
		out: &AttributeResolution{
			Status:        "resolved",
			Source:        "attribute_templates",
			CategoryID:    8218,
			TemplateCount: 1,
			ResolvedCount: 1,
			ResolvedAttributes: []ResolvedAttribute{{
				Name:             "Material",
				Value:            "Polyester",
				AttributeID:      160,
				AttributeValueID: &valueID,
			}},
		},
	}
	saleValueID := 103
	saleInner := &countingSaleAttributeResolver{
		out: &SaleAttributeResolution{
			Status:                 "resolved",
			Source:                 "sale_attribute_templates",
			CategoryID:             8218,
			PrimaryAttributeID:     27,
			PrimarySourceDimension: "Color",
			SKCAttributes: []ResolvedSaleAttribute{{
				Scope:            "skc",
				Name:             "Color",
				Value:            "White",
				AttributeID:      27,
				AttributeValueID: &saleValueID,
			}},
			skcValueAssignments: map[string]ResolvedSaleAttribute{
				"white": {
					Scope:            "skc",
					Name:             "Color",
					Value:            "White",
					AttributeID:      27,
					AttributeValueID: &saleValueID,
				},
			},
		},
	}
	firstCategory := NewCachedCategoryResolver(categoryInner, store)
	firstAttribute := NewCachedAttributeResolver(attributeInner, store)
	firstSale := NewCachedSaleAttributeResolver(saleInner, store)
	firstAssembler := NewAssembler(AssemblerConfig{
		CategoryResolver:      firstCategory,
		AttributeResolver:     firstAttribute,
		SaleAttributeResolver: firstSale,
	})

	first := firstAssembler.Build(req, product, nil)
	if categoryInner.calls != 1 || attributeInner.calls != 1 || saleInner.calls != 1 {
		t.Fatalf("first build resolver calls = category:%d attribute:%d sale:%d, want 1/1/1", categoryInner.calls, attributeInner.calls, saleInner.calls)
	}
	firstCategory.(CategoryResolutionCache).RememberCategoryResolution(req, product, first, first.CategoryResolution)
	firstAttribute.(AttributeResolutionCache).RememberAttributeResolution(req, product, first, first.AttributeResolution)
	firstSale.(SaleAttributeResolutionCache).RememberSaleAttributeResolution(req, product, first, first.SaleAttributeResolution)

	secondCategoryInner := &countingCategoryResolver{out: categoryInner.out}
	secondAttributeInner := &countingAttributeResolver{out: attributeInner.out}
	secondSaleInner := &countingSaleAttributeResolver{out: saleInner.out}
	second := NewAssembler(AssemblerConfig{
		CategoryResolver:      NewCachedCategoryResolver(secondCategoryInner, store),
		AttributeResolver:     NewCachedAttributeResolver(secondAttributeInner, store),
		SaleAttributeResolver: NewCachedSaleAttributeResolver(secondSaleInner, store),
	}).Build(req, product, nil)

	if secondCategoryInner.calls != 0 || secondAttributeInner.calls != 0 || secondSaleInner.calls != 0 {
		t.Fatalf("second build resolver calls = category:%d attribute:%d sale:%d, want 0/0/0", secondCategoryInner.calls, secondAttributeInner.calls, secondSaleInner.calls)
	}
	if second.CategoryResolution == nil || second.CategoryResolution.Cache == nil || second.CategoryResolution.Cache.Source != "manual_cache" {
		t.Fatalf("second category cache = %+v, want manual_cache hit", second.CategoryResolution)
	}
	if second.CategoryResolution.Cache.HitSource != ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("second category hit source = %q, want %q", second.CategoryResolution.Cache.HitSource, ResolutionCacheHitSourcePersistentManualCache)
	}
	if second.AttributeResolution == nil || second.AttributeResolution.Cache == nil || second.AttributeResolution.Cache.Source != "manual_cache" {
		t.Fatalf("second attribute cache = %+v, want manual_cache hit", second.AttributeResolution)
	}
	if second.AttributeResolution.Cache.HitSource != ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("second attribute hit source = %q, want %q", second.AttributeResolution.Cache.HitSource, ResolutionCacheHitSourcePersistentManualCache)
	}
	if second.SaleAttributeResolution == nil || second.SaleAttributeResolution.Cache == nil || second.SaleAttributeResolution.Cache.Source != "manual_cache" {
		t.Fatalf("second sale attribute cache = %+v, want manual_cache hit", second.SaleAttributeResolution)
	}
	if second.SaleAttributeResolution.Cache.HitSource != ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("second sale attribute hit source = %q, want %q", second.SaleAttributeResolution.Cache.HitSource, ResolutionCacheHitSourcePersistentManualCache)
	}
}

func testCanonicalProduct() *canonical.Product {
	return &canonical.Product{
		Title: "Running Shoes",
		Variants: []canonical.Variant{
			{
				SKU: "SKU-RED-42",
				Attributes: map[string]canonical.Attribute{
					"color": {Value: "Red"},
					"size":  {Value: "42"},
				},
				Stock: 10,
				Images: []canonical.Image{
					{URL: "red-42.jpg"},
				},
			},
			{
				SKU: "SKU-RED-43",
				Attributes: map[string]canonical.Attribute{
					"color": {Value: "Red"},
					"size":  {Value: "43"},
				},
				Stock: 8,
				Images: []canonical.Image{
					{URL: "red-43.jpg"},
				},
			},
			{
				SKU: "SKU-BLUE-42",
				Attributes: map[string]canonical.Attribute{
					"color": {Value: "Blue"},
					"size":  {Value: "42"},
				},
				Stock: 6,
				Images: []canonical.Image{
					{URL: "blue-42.jpg"},
				},
			},
		},
	}
}

func boolPointer(value bool) *bool {
	return &value
}

func TestFilterSaleScopeAttributesIncludesChineseSaleAttributeNames(t *testing.T) {
	attributes := []sheinattribute.AttributeInfo{
		{AttributeID: 27, AttributeName: "颜色分类"},
		{AttributeID: 28, AttributeName: "尺码"},
		{AttributeID: 29, AttributeName: "材质"},
	}

	filtered := filterSaleScopeAttributes(attributes)
	if len(filtered) != 2 {
		t.Fatalf("filtered count = %d, want 2", len(filtered))
	}
	if filtered[0].AttributeID != 27 || filtered[1].AttributeID != 28 {
		t.Fatalf("filtered attribute ids = %d,%d, want 27,28", filtered[0].AttributeID, filtered[1].AttributeID)
	}
}
