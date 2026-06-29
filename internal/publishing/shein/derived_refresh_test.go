package shein

import (
	"testing"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
	sheinproduct "task-processor/internal/shein/api/product"
)

type refreshDerivedStubAttributeResolver struct{}

func (refreshDerivedStubAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *AttributeResolution {
	return &AttributeResolution{
		Status: "resolved",
		ResolvedAttributes: []ResolvedAttribute{{
			Name:        "Upper Material",
			Value:       "Mesh Fabric",
			AttributeID: 112,
		}},
	}
}

type refreshDerivedStubSaleResolver struct{}

func (refreshDerivedStubSaleResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	valueID := 2493
	return &SaleAttributeResolution{
		Status:                  "resolved",
		PrimaryAttributeID:      27,
		PrimarySourceDimension:  "颜色",
		RecommendCategoryReview: false,
		CategoryReviewReason:    "",
		skcValueAssignments:     map[string]ResolvedSaleAttribute{"黑色": {Scope: "skc", Name: "Color", Value: "Black", AttributeID: 27, AttributeValueID: &valueID}},
		SelectionSummary:        []string{"主销售属性使用源维度 颜色 映射到 Color"},
	}
}

type refreshDerivedStubSplitSaleResolver struct{}

func (refreshDerivedStubSplitSaleResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	valueID := 2493
	return &SaleAttributeResolution{
		Status:                   "resolved",
		PrimaryAttributeID:       27,
		PrimarySourceDimension:   "Color",
		SecondarySourceDimension: "Size",
		RecommendCategoryReview:  false,
		CategoryReviewReason:     "",
		skcValueAssignments: map[string]ResolvedSaleAttribute{
			"white": {Scope: "skc", Name: "Color", Value: "white", AttributeID: 27, AttributeValueID: &valueID},
		},
		SelectionSummary: []string{"主销售属性使用源维度 Color 映射到 Color"},
	}
}

type refreshDerivedStubSizeSaleResolver struct{}

func (refreshDerivedStubSizeSaleResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	sizeMValueID := 417
	sizeLValueID := 568
	return &SaleAttributeResolution{
		Status:                   "resolved",
		SecondarySourceDimension: "Size",
		SecondaryAttributeID:     87,
		SKUValueAssignments: map[string]ResolvedSaleAttribute{
			normalizeText("M"): {Value: "M", AttributeID: 87, AttributeValueID: &sizeMValueID},
			normalizeText("L"): {Value: "L", AttributeID: 87, AttributeValueID: &sizeLValueID},
		},
	}
}

func TestRefreshDerivedStateRecomputesSaleAttributesAndClearsSuggestion(t *testing.T) {
	t.Parallel()

	canonical := &canonical.Product{
		Title: "Bottle",
		Attributes: map[string]canonical.Attribute{
			"颜色": {Value: "黑色"},
		},
		Variants: []canonical.Variant{{
			SKU: "SKU-1",
			Attributes: map[string]canonical.Attribute{
				"颜色": {Value: "黑色"},
			},
		}},
	}
	pkg := &Package{
		CategoryID: 3221,
		CategoryResolution: &CategoryResolution{
			Status:     "resolved",
			CategoryID: 3221,
			SuggestedCategory: &CategorySuggestion{
				CategoryID: 999,
			},
		},
		ProductAttributes: []common.Attribute{{Name: "颜色", Value: "黑色"}},
		RequestDraft:      &RequestDraft{},
	}

	RefreshDerivedState(
		&BuildRequest{Country: "US"},
		canonical,
		nil,
		pkg,
		nil,
		refreshDerivedStubAttributeResolver{},
		refreshDerivedStubSaleResolver{},
		PricingPolicy{},
	)

	if pkg.AttributeResolution == nil || pkg.AttributeResolution.Status != "resolved" {
		t.Fatalf("attribute resolution = %+v", pkg.AttributeResolution)
	}
	if pkg.SaleAttributeResolution == nil || pkg.SaleAttributeResolution.Status != "resolved" {
		t.Fatalf("sale resolution = %+v", pkg.SaleAttributeResolution)
	}
	if pkg.SaleAttributeResolution.RecommendCategoryReview {
		t.Fatalf("sale resolution recommend_category_review = true, want false")
	}
	if pkg.CategoryResolution == nil || pkg.CategoryResolution.SuggestedCategory != nil {
		t.Fatalf("category suggestion = %+v, want nil", pkg.CategoryResolution)
	}
	if pkg.RequestDraft == nil || len(pkg.RequestDraft.SKCList) != 1 {
		t.Fatalf("request draft skc_list = %+v", pkg.RequestDraft)
	}
	if pkg.RequestDraft.SKCList[0].SaleAttribute == nil || pkg.RequestDraft.SKCList[0].SaleAttribute.AttributeID != 27 {
		t.Fatalf("request draft skc sale attribute = %+v", pkg.RequestDraft.SKCList[0].SaleAttribute)
	}
	if pkg.PreviewProduct == nil || len(pkg.PreviewProduct.SKCList) != 1 {
		t.Fatalf("preview product = %+v", pkg.PreviewProduct)
	}
}

func TestRefreshDerivedStatePreservesVariantSpecificSKCImagesWhenFallbackSplitsGroup(t *testing.T) {
	t.Parallel()

	canonicalProduct := &canonical.Product{
		Title: "Backpack",
		Variants: []canonical.Variant{
			{
				SKU: "SKU-WHITE-30X40",
				Attributes: map[string]canonical.Attribute{
					"Color":          {Value: "white"},
					"Size":           {Value: "30x40"},
					"source_sds_sku": {Value: "SDS-30X40"},
				},
			},
			{
				SKU: "SKU-WHITE-35X50",
				Attributes: map[string]canonical.Attribute{
					"Color":          {Value: "white"},
					"Size":           {Value: "35x50"},
					"source_sds_sku": {Value: "SDS-35X50"},
				},
			},
		},
	}

	pkg := &Package{
		ProductNameEn: "Backpack",
		Images:        &common.ImageSet{MainImage: "https://cdn.example.com/spu-main.jpg"},
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SkcName:      "Backpack - white",
				SupplierCode: "SKU-WHITE-30X40",
				ImageInfo: &ImageDraft{
					MainImage: "https://cdn.example.com/group-main.jpg",
				},
				SKUList: []SKUDraft{
					{
						SupplierSKU: "SKU-WHITE-30X40",
						Attributes: map[string]string{
							"Color":          "white",
							"Size":           "30x40",
							"source_sds_sku": "SDS-30X40",
						},
						MainImage: "https://cdn.example.com/30x40-main.jpg",
					},
					{
						SupplierSKU: "SKU-WHITE-35X50",
						Attributes: map[string]string{
							"Color":          "white",
							"Size":           "35x50",
							"source_sds_sku": "SDS-35X50",
						},
						MainImage: "https://cdn.example.com/35x50-main.jpg",
					},
				},
			}},
		},
		SkcList: []SKCPackage{{
			SkcName:      "Backpack - white",
			SupplierCode: "SKU-WHITE-30X40",
			MainImageURL: "https://cdn.example.com/group-main.jpg",
			SKUs: []common.Variant{
				{SKU: "SKU-WHITE-30X40", Attributes: map[string]string{"source_sds_sku": "SDS-30X40", "Color": "white", "Size": "30x40"}},
				{SKU: "SKU-WHITE-35X50", Attributes: map[string]string{"source_sds_sku": "SDS-35X50", "Color": "white", "Size": "35x50"}},
			},
		}},
	}

	RefreshDerivedState(
		&BuildRequest{Country: "US"},
		canonicalProduct,
		nil,
		pkg,
		nil,
		nil,
		refreshDerivedStubSplitSaleResolver{},
		PricingPolicy{},
	)

	if len(pkg.RequestDraft.SKCList) != 2 {
		t.Fatalf("request draft skc count = %d, want 2", len(pkg.RequestDraft.SKCList))
	}
	if pkg.RequestDraft.SKCList[0].ImageInfo == nil || pkg.RequestDraft.SKCList[0].ImageInfo.MainImage != "https://cdn.example.com/30x40-main.jpg" {
		t.Fatalf("first request draft skc image = %+v, want 30x40 image", pkg.RequestDraft.SKCList[0].ImageInfo)
	}
	if pkg.RequestDraft.SKCList[1].ImageInfo == nil || pkg.RequestDraft.SKCList[1].ImageInfo.MainImage != "https://cdn.example.com/35x50-main.jpg" {
		t.Fatalf("second request draft skc image = %+v, want 35x50 image", pkg.RequestDraft.SKCList[1].ImageInfo)
	}
	if pkg.SkcList[0].MainImageURL != "https://cdn.example.com/30x40-main.jpg" {
		t.Fatalf("first package skc image = %q, want 30x40 image", pkg.SkcList[0].MainImageURL)
	}
	if pkg.SkcList[1].MainImageURL != "https://cdn.example.com/35x50-main.jpg" {
		t.Fatalf("second package skc image = %q, want 35x50 image", pkg.SkcList[1].MainImageURL)
	}
	if pkg.PreviewProduct == nil || len(pkg.PreviewProduct.SKCList) != 2 {
		t.Fatalf("preview product skcs = %+v, want 2", pkg.PreviewProduct)
	}
	if got := pkg.PreviewProduct.SKCList[0].ImageInfo.ImageInfoList[0].ImageURL; got != "https://cdn.example.com/30x40-main.jpg" {
		t.Fatalf("first preview skc image = %q, want 30x40 image", got)
	}
	if got := pkg.PreviewProduct.SKCList[1].ImageInfo.ImageInfoList[0].ImageURL; got != "https://cdn.example.com/35x50-main.jpg" {
		t.Fatalf("second preview skc image = %q, want 35x50 image", got)
	}
}

func TestRefreshDerivedStateRebuildsSizeAttributesIntoPreviewPayload(t *testing.T) {
	t.Parallel()

	canonicalProduct := &canonical.Product{
		Title: "Oversized Tee",
		Images: []canonical.Image{
			{URL: "https://example.com/main.jpg"},
		},
		Variants: []canonical.Variant{
			{
				SKU:        "SKU-M",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "M"}},
				Stock:      5,
				IsDefault:  true,
			},
			{
				SKU:        "SKU-L",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "L"}},
				Stock:      5,
			},
		},
	}
	productSize := `[[{"content":"尺码","remark":""},{"content":"肩宽(cm/in)","remark":""},{"content":"胸围(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"55cm/21.7in","remark":""},{"content":"112cm /44.1in","remark":""}],[{"content":"L","remark":""},{"content":"58cm/22.8in","remark":""},{"content":"118cm /46.5in","remark":""}]]`
	pkg := &Package{
		ProductNameEn: "Oversized Tee",
		RequestDraft:  &RequestDraft{},
	}

	RefreshDerivedState(
		&BuildRequest{Country: "US", Language: "en", ProductSize: productSize},
		canonicalProduct,
		nil,
		pkg,
		nil,
		nil,
		refreshDerivedStubSizeSaleResolver{},
		PricingPolicy{},
	)

	if pkg.RequestDraft == nil {
		t.Fatal("request draft = nil")
	}
	if got := pkg.RequestDraft.SizeAttributeList; len(got) != 4 {
		t.Fatalf("request draft size_attribute_list = %#v, want 4 items", got)
	}
	if pkg.PreviewProduct == nil {
		t.Fatal("preview product = nil")
	}
	got := pkg.PreviewProduct.SizeAttributeList
	if len(got) != 4 {
		t.Fatalf("preview size_attribute_list = %#v, want 4 items", got)
	}
	if got[0].AttributeID != 10 || got[0].AttributeExtraValue != "55" || got[0].RelateSaleAttributeID != 87 || got[0].RelateSaleAttributeValueID != 417 {
		t.Fatalf("first preview size attribute = %#v", got[0])
	}
	if got[3].AttributeID != 15 || got[3].AttributeExtraValue != "118" || got[3].RelateSaleAttributeValueID != 568 {
		t.Fatalf("last preview size attribute = %#v", got[3])
	}
}

func TestRefreshDerivedStateNilRequestDoesNotPanicOnProductSizeAccess(t *testing.T) {
	t.Parallel()

	canonicalProduct := &canonical.Product{
		Title: "Oversized Tee",
		Variants: []canonical.Variant{
			{
				SKU:        "SKU-M",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "M"}},
				Stock:      5,
				IsDefault:  true,
			},
		},
	}
	pkg := &Package{
		RequestDraft: &RequestDraft{},
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("RefreshDerivedState(nil, ...) panicked: %v", recovered)
		}
	}()

	RefreshDerivedState(
		nil,
		canonicalProduct,
		nil,
		pkg,
		nil,
		nil,
		refreshDerivedStubSizeSaleResolver{},
		PricingPolicy{},
	)

	if pkg.RequestDraft == nil {
		t.Fatal("request draft = nil")
	}
	if pkg.PreviewProduct == nil {
		t.Fatal("preview product = nil")
	}
}

func TestRefreshDerivedStateClearsStaleSizeAttributesWhenProductSizeInvalid(t *testing.T) {
	t.Parallel()

	canonicalProduct := &canonical.Product{
		Title: "Oversized Tee",
		Variants: []canonical.Variant{
			{
				SKU:        "SKU-M",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "M"}},
				Stock:      5,
				IsDefault:  true,
			},
			{
				SKU:        "SKU-L",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "L"}},
				Stock:      5,
			},
		},
	}
	stale := []ResolvedSaleAttribute{
		{AttributeID: 87, AttributeValueID: intPtr(417), Value: "M"},
	}
	cases := []struct {
		name        string
		productSize string
	}{
		{name: "empty", productSize: ""},
		{name: "malformed", productSize: `{"broken":true}`},
		{name: "unsupported", productSize: `[[{"content":"尺码","remark":""},{"content":"腰围(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"70cm/27.6in","remark":""}]]`},
		{name: "unmatched", productSize: `[[{"content":"尺码","remark":""},{"content":"肩宽(cm/in)","remark":""}],[{"content":"XL","remark":""},{"content":"55cm/21.7in","remark":""}]]`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			pkg := &Package{
				RequestDraft: &RequestDraft{
					SizeAttributeList: []sheinproduct.SizeAttribute{{
						AttributeID:                10,
						AttributeExtraValue:        "stale",
						RelateSaleAttributeID:      87,
						RelateSaleAttributeValueID: 999,
					}},
				},
				PreviewProduct: &sheinproduct.Product{
					SizeAttributeList: []sheinproduct.SizeAttribute{{
						AttributeID:                10,
						AttributeExtraValue:        "stale",
						RelateSaleAttributeID:      87,
						RelateSaleAttributeValueID: 999,
					}},
				},
				SaleAttributeResolution: &SaleAttributeResolution{
					SecondarySourceDimension: "Size",
					SecondaryAttributeID:     87,
					SKUValueAssignments: map[string]ResolvedSaleAttribute{
						normalizeText("M"): stale[0],
					},
				},
			}

			RefreshDerivedState(
				&BuildRequest{Country: "US", Language: "en", ProductSize: tc.productSize},
				canonicalProduct,
				nil,
				pkg,
				nil,
				nil,
				refreshDerivedStubSizeSaleResolver{},
				PricingPolicy{},
			)

			if got := pkg.RequestDraft.SizeAttributeList; len(got) != 0 {
				t.Fatalf("request draft size_attribute_list = %#v, want cleared", got)
			}
			if pkg.PreviewProduct == nil {
				t.Fatal("preview product = nil")
			}
			if got := pkg.PreviewProduct.SizeAttributeList; len(got) != 0 {
				t.Fatalf("preview size_attribute_list = %#v, want cleared", got)
			}
		})
	}
}

func intPtr(value int) *int {
	return &value
}
