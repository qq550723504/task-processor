package shein

import sheinpub "task-processor/internal/publishing/shein"

func BuildAppliedChangesPreview(before, after *sheinpub.Package) *RevisionDiffPreview {
	if before == nil || after == nil {
		return nil
	}
	preview := BuildRevisionDiffPreview(before, BuildMinimalRevisionSkeleton(BuildEditorRevisionSkeleton(
		after,
		BuildCategoryResolutionPatch(after),
		BuildAttributeResolutionPatch(after),
		BuildSaleAttributeResolutionPatch(after),
		BuildEditorSKCPatches(after),
	)))
	if preview == nil {
		preview = &RevisionDiffPreview{}
	}
	appendAppliedSaleAttributeReviewChanges(&preview.Changes, before, after)
	preview.ChangeCount = len(preview.Changes)
	return preview
}

func appendAppliedSaleAttributeReviewChanges(changes *[]RevisionFieldChange, before, after *sheinpub.Package) {
	if changes == nil || before == nil || after == nil {
		return
	}
	beforeRecommend := saleAttributeRecommendCategoryReview(before)
	afterRecommend := saleAttributeRecommendCategoryReview(after)
	if beforeRecommend != afterRecommend {
		*changes = append(*changes, RevisionFieldChange{
			FieldPath: "shein.sale_attribute_resolution.recommend_category_review",
			Label:     "类目复核状态",
			Before:    beforeRecommend,
			After:     afterRecommend,
		})
	}
	beforeReason := saleAttributeCategoryReviewReason(before)
	afterReason := saleAttributeCategoryReviewReason(after)
	if beforeReason != afterReason {
		*changes = append(*changes, RevisionFieldChange{
			FieldPath: "shein.sale_attribute_resolution.category_review_reason",
			Label:     "类目复核原因",
			Before:    beforeReason,
			After:     afterReason,
		})
	}
}

func saleAttributeRecommendCategoryReview(pkg *sheinpub.Package) bool {
	return pkg != nil && pkg.SaleAttributeResolution != nil && pkg.SaleAttributeResolution.RecommendCategoryReview
}

func saleAttributeCategoryReviewReason(pkg *sheinpub.Package) string {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return ""
	}
	return pkg.SaleAttributeResolution.CategoryReviewReason
}
