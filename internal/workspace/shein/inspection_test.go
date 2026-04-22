package shein

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSaleAttributePayloadIncludesCategoryReviewSignal(t *testing.T) {
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:                  "partial",
			Source:                  "sale_attribute_templates",
			RecommendCategoryReview: true,
			CategoryReviewReason:    "当前类目销售属性模板未提供可承接款式/型号的销售属性字段",
		},
	}

	payload := BuildSaleAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if !payload.RecommendCategoryReview {
		t.Fatalf("recommend_category_review = %v, want true", payload.RecommendCategoryReview)
	}
	if payload.CategoryReviewReason != "当前类目销售属性模板未提供可承接款式/型号的销售属性字段" {
		t.Fatalf("category_review_reason = %q", payload.CategoryReviewReason)
	}
}
