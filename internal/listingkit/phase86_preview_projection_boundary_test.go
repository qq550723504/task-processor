package listingkit

import "testing"

func TestSheinPreviewProjectionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("preview review summary delegates to workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_builder_shein_review_summary.go", "buildSheinPreviewReviewSummary")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein_review_summary.go", "buildSheinPreviewReviewSummary")

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.BuildPreviewReviewSummary(pkg)",
		})
		assertSourceExcludesAll(t, source, []string{
			"pkg.ReviewNotes",
			"pkg.Inspection.Summary",
			"needsReview := len(pkg.ReviewNotes) > 0",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"BuildPreviewReviewSummary",
		})
	})

	t.Run("final review image and sku projection delegates to workspace", func(t *testing.T) {
		t.Parallel()

		imageSource := readNamedFunctionSource(t, "preview_builder_shein_final_review_images.go", "buildSheinFinalReviewImages")
		imageCalls := readNamedFunctionCallNames(t, "preview_builder_shein_final_review_images.go", "buildSheinFinalReviewImages")
		assertSourceContainsAll(t, imageSource, []string{
			"return sheinworkspace.BuildFinalReviewImages(draft, finalDraft, product)",
		})
		assertSourceExcludesAll(t, imageSource, []string{
			"sheinproduct.CollectSizeMapImageURLs(product)",
			"mergeSheinFinalReviewImage(",
		})
		assertFunctionCallsContainAll(t, imageCalls, []string{
			"BuildFinalReviewImages",
		})

		skusSource := readNamedFunctionSource(t, "preview_builder_shein_final_review_skus.go", "buildSheinFinalReviewSKUs")
		skusCalls := readNamedFunctionCallNames(t, "preview_builder_shein_final_review_skus.go", "buildSheinFinalReviewSKUs")
		assertSourceContainsAll(t, skusSource, []string{
			"return sheinworkspace.BuildFinalReviewSKUs(draft)",
		})
		assertSourceExcludesAll(t, skusSource, []string{
			"for _, skc := range draft.SKCList",
			"buildSheinFinalReviewSKU(skc.SupplierCode, sku)",
		})
		assertFunctionCallsContainAll(t, skusCalls, []string{
			"BuildFinalReviewSKUs",
		})

		skuSource := readNamedFunctionSource(t, "preview_builder_shein_final_review_skus.go", "buildSheinFinalReviewSKU")
		skuCalls := readNamedFunctionCallNames(t, "preview_builder_shein_final_review_skus.go", "buildSheinFinalReviewSKU")
		assertSourceContainsAll(t, skuSource, []string{
			"return sheinworkspace.BuildFinalReviewSKU(supplierCode, sku)",
		})
		assertSourceExcludesAll(t, skuSource, []string{
			"parseMoney(sku.BasePrice)",
			"normalizeSheinFinalReviewAttributeName(attr.Name)",
		})
		assertFunctionCallsContainAll(t, skuCalls, []string{
			"BuildFinalReviewSKU",
		})
	})
}
