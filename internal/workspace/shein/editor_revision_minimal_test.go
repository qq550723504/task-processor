package shein

import "testing"

func TestPruneSaleAttributeResolutionPatchPreservesCategoryReviewSignal(t *testing.T) {
	t.Parallel()

	recommend := true
	reason := "当前类目销售属性模板未提供可承接款式/型号的销售属性字段"
	patch := &SaleAttributeResolutionPatch{
		RecommendCategoryReview: &recommend,
		CategoryReviewReason:    &reason,
	}

	pruned := PruneSaleAttributeResolutionPatch(patch)
	if pruned == nil {
		t.Fatal("expected pruned patch")
	}
	if pruned.RecommendCategoryReview == nil || !*pruned.RecommendCategoryReview {
		t.Fatalf("recommend_category_review = %#v, want true", pruned.RecommendCategoryReview)
	}
	if pruned.CategoryReviewReason == nil || *pruned.CategoryReviewReason != reason {
		t.Fatalf("category_review_reason = %#v", pruned.CategoryReviewReason)
	}
}
