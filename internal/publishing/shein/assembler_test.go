package shein

import (
	"testing"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

type stubAttributeAPI struct {
	templates *sheinattribute.AttributeTemplateInfo
}

type assemblerStubCategoryAPI struct {
	info *sheincategory.CategoryInfo
}

func (s assemblerStubCategoryAPI) GetCategory(categoryID int) (*sheincategory.CategoryInfo, error) {
	if s.info == nil {
		return nil, nil
	}
	info := *s.info
	info.CategoryID = categoryID
	return &info, nil
}

func (assemblerStubCategoryAPI) SuggestCategoryByText(string) (*sheincategory.SuggestCategoryResponse, error) {
	return nil, nil
}

func (s stubAttributeAPI) GetAttributeTemplates(categoryID int) (*sheinattribute.AttributeTemplateInfo, error) {
	return s.templates, nil
}

func TestBuildRequestSKCsGroupsVariantsByColor(t *testing.T) {
	canonical := testCanonicalProduct()
	variants := common.BuildVariants(canonical)
	groups := buildVariantGroups(variants, &common.ImageSet{MainImage: "main.jpg"}, &SaleAttributeResolution{
		PrimarySourceDimension: "color",
	})

	if len(groups) != 2 {
		t.Fatalf("group count = %d, want 2", len(groups))
	}
	if groups[0].skcName != "Red" {
		t.Fatalf("first group skc name = %q, want Red", groups[0].skcName)
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

	requestSKCs := buildRequestSKCs(groups, &common.ImageSet{MainImage: "main.jpg"}, common.DefaultSites("US"), canonical)
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
	groups := buildVariantGroups(variants, images, &SaleAttributeResolution{
		PrimarySourceDimension: "color",
	})
	pkg := &Package{
		CategoryID: 100,
		SkcList:    buildSKCs(groups),
		RequestDraft: &RequestDraft{
			SKCList: buildRequestSKCs(groups, images, common.DefaultSites("US"), canonical),
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
	canonical := &productenrich.CanonicalProduct{
		Title:       "Minimal Product",
		Description: "fallback variant",
		Attributes: map[string]productenrich.CanonicalAttribute{
			"color": {Value: "Black"},
			"size":  {Value: "One Size"},
			"price": {Value: "19.9"},
		},
		Images: []productenrich.CanonicalImage{{URL: "main.jpg"}},
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
	canonical := &productenrich.CanonicalProduct{
		Title:       "Fallback Matrix Product",
		Description: "fallback matrix",
		VariantDimensions: []productenrich.ScrapedVariantDimension{
			{Name: "color", Values: []string{"Red", "Blue"}},
			{Name: "size", Values: []string{"42", "43"}},
		},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"color": {Value: "Red, Blue"},
			"size":  {Value: "42/43"},
		},
		Images: []productenrich.CanonicalImage{{URL: "main.jpg"}},
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
	if pkg.RequestDraft.SKCList[0].SkcName != "Red" || pkg.RequestDraft.SKCList[1].SkcName != "Blue" {
		t.Fatalf("skc names = %q, %q; want Red/Blue", pkg.RequestDraft.SKCList[0].SkcName, pkg.RequestDraft.SKCList[1].SkcName)
	}
}

func testCanonicalProduct() *productenrich.CanonicalProduct {
	return &productenrich.CanonicalProduct{
		Title: "Running Shoes",
		Variants: []productenrich.CanonicalVariant{
			{
				SKU: "SKU-RED-42",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"color": {Value: "Red"},
					"size":  {Value: "42"},
				},
				Stock: 10,
				Images: []productenrich.CanonicalImage{
					{URL: "red-42.jpg"},
				},
			},
			{
				SKU: "SKU-RED-43",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"color": {Value: "Red"},
					"size":  {Value: "43"},
				},
				Stock: 8,
				Images: []productenrich.CanonicalImage{
					{URL: "red-43.jpg"},
				},
			},
			{
				SKU: "SKU-BLUE-42",
				Attributes: map[string]productenrich.CanonicalAttribute{
					"color": {Value: "Blue"},
					"size":  {Value: "42"},
				},
				Stock: 6,
				Images: []productenrich.CanonicalImage{
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
