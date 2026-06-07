package shein

import (
	"testing"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
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
		Status:                  "resolved",
		PrimaryAttributeID:      27,
		PrimarySourceDimension:  "Color",
		SecondarySourceDimension:"Size",
		RecommendCategoryReview: false,
		CategoryReviewReason:    "",
		skcValueAssignments: map[string]ResolvedSaleAttribute{
			"white": {Scope: "skc", Name: "Color", Value: "white", AttributeID: 27, AttributeValueID: &valueID},
		},
		SelectionSummary: []string{"主销售属性使用源维度 Color 映射到 Color"},
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
		Images: &common.ImageSet{MainImage: "https://cdn.example.com/spu-main.jpg"},
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
