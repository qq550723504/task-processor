package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
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

func TestBuildAttributePayloadPrefersPendingAttributesFromResolution(t *testing.T) {
	pkg := &sheinpub.Package{
		ProductAttributes: []common.Attribute{{Name: "Material", Value: "Cotton"}},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status:            "partial",
			PendingAttributes: []common.Attribute{{Name: "Width (cm)"}},
		},
	}

	payload := BuildAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if len(payload.PendingAttributes) != 1 {
		t.Fatalf("pending attributes = %#v, want 1", payload.PendingAttributes)
	}
	if payload.PendingAttributes[0].Name != "Width (cm)" {
		t.Fatalf("pending attribute name = %q, want Width (cm)", payload.PendingAttributes[0].Name)
	}
}
