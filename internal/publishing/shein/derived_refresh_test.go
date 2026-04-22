package shein

import (
	"testing"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
)

type refreshDerivedStubAttributeResolver struct{}

func (refreshDerivedStubAttributeResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *AttributeResolution {
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

func (refreshDerivedStubSaleResolver) Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *SaleAttributeResolution {
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

func TestRefreshDerivedStateRecomputesSaleAttributesAndClearsSuggestion(t *testing.T) {
	t.Parallel()

	canonical := &productenrich.CanonicalProduct{
		Title: "Bottle",
		Attributes: map[string]productenrich.CanonicalAttribute{
			"颜色": {Value: "黑色"},
		},
		Variants: []productenrich.CanonicalVariant{{
			SKU: "SKU-1",
			Attributes: map[string]productenrich.CanonicalAttribute{
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
