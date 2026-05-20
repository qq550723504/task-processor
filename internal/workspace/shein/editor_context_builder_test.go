package shein

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSaleAttributeResolutionPatchIncludesCategoryReviewSignal(t *testing.T) {
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:                  "partial",
			Source:                  "sale_attribute_templates",
			RecommendCategoryReview: true,
			CategoryReviewReason:    "当前类目销售属性模板未提供可承接款式/型号的销售属性字段",
			ReviewNotes:             []string{"note"},
		},
	}

	patch := BuildSaleAttributeResolutionPatch(pkg)
	if patch == nil {
		t.Fatal("expected patch")
	}
	if patch.RecommendCategoryReview == nil || !*patch.RecommendCategoryReview {
		t.Fatalf("recommend_category_review = %#v, want true", patch.RecommendCategoryReview)
	}
	if patch.CategoryReviewReason == nil || *patch.CategoryReviewReason != "当前类目销售属性模板未提供可承接款式/型号的销售属性字段" {
		t.Fatalf("category_review_reason = %#v", patch.CategoryReviewReason)
	}
}

func TestBuildSaleAttributeResolutionPatchDowngradesResolvedStatusWhenValueIDsAreMissing(t *testing.T) {
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001466,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:       "skc",
				Name:        "Plug(Voltage)",
				Value:       "white",
				AttributeID: 1001466,
			}},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{SupplierCode: "SKC-1"}},
		},
	}

	patch := BuildSaleAttributeResolutionPatch(pkg)
	if patch == nil || patch.Status == nil {
		t.Fatalf("patch = %#v, want status", patch)
	}
	if *patch.Status != "partial" {
		t.Fatalf("patch status = %q, want partial", *patch.Status)
	}
}
