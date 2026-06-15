package listingkit

import "testing"

func TestGenerationReviewPatchSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "generation_review_patch.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func buildGenerationReviewSessionPatch(",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func generationReviewSessionFocusChanged(",
		"func diffGenerationReviewSections(",
		"func diffGenerationReviewSlots(",
		"func diffGenerationPlatformCards(",
		"func diffPlatformAssetRenderPreviews(",
		"func cloneGenerationWorkQueueSummary(",
		"func buildGenerationReviewQueuePatch(",
		"func generationReviewQueueSummaryChanged(",
		"func generationReviewSummaryChanged(",
		"func generationReviewOverviewChanged(",
		"func buildGenerationReviewCardsPatch(",
		"func buildGenerationReviewPreviewsPatch(",
		"func equalReviewPatchValue(",
		"func isGenerationReviewSessionPatchEmpty(",
	})

	supportSource := readTaskGenerationSourceFile(t, "generation_review_patch_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func generationReviewSessionFocusChanged(",
		"func diffGenerationReviewSections(",
		"func diffGenerationReviewSlots(",
		"func diffGenerationPlatformCards(",
		"func diffPlatformAssetRenderPreviews(",
		"func cloneGenerationWorkQueueSummary(",
		"func buildGenerationReviewQueuePatch(",
		"func generationReviewQueueSummaryChanged(",
		"func generationReviewSummaryChanged(",
		"func generationReviewOverviewChanged(",
		"func buildGenerationReviewCardsPatch(",
		"func buildGenerationReviewPreviewsPatch(",
		"func equalReviewPatchValue(",
		"func isGenerationReviewSessionPatchEmpty(",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func buildGenerationReviewSessionPatch(",
	})
}
