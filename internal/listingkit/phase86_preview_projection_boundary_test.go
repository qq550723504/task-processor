package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinPreviewProjectionBoundary(t *testing.T) {
	t.Parallel()

	t.Run("preview review summary delegates to workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_builder_shein.go", "buildSheinPreviewPayloadInput")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein.go", "buildSheinPreviewPayloadInput")
		fileSource, err := os.ReadFile("preview_builder_shein.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein.go) error = %v", err)
		}

		assertSourceContainsAll(t, source, []string{
			"needsReview, summary := sheinworkspace.BuildPreviewReviewSummary(pkg)",
		})
		if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("preview_builder_shein.go should call marketplace SHEIN workspace directly")
		}
		assertSourceExcludesAll(t, source, []string{
			"pkg.ReviewNotes",
			"pkg.Inspection.Summary",
			"needsReview := len(pkg.ReviewNotes) > 0",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"BuildPreviewReviewSummary",
		})
		assertFileAbsent(t, "preview_builder_shein_review_summary.go")
	})

	t.Run("final review image and sku projection delegates to workspace", func(t *testing.T) {
		t.Parallel()

		imageSource := readNamedFunctionSource(t, "preview_builder_shein_final_review_images.go", "buildSheinFinalReviewImages")
		imageCalls := readNamedFunctionCallNames(t, "preview_builder_shein_final_review_images.go", "buildSheinFinalReviewImages")
		imageFileSource, err := os.ReadFile("preview_builder_shein_final_review_images.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein_final_review_images.go) error = %v", err)
		}
		assertSourceContainsAll(t, imageSource, []string{
			"return sheinworkspace.BuildFinalReviewImages(draft, finalDraft, product)",
		})
		if !strings.Contains(string(imageFileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("preview_builder_shein_final_review_images.go should call marketplace SHEIN workspace directly")
		}
		assertSourceExcludesAll(t, imageSource, []string{
			"sheinproduct.CollectSizeMapImageURLs(product)",
			"mergeSheinFinalReviewImage(",
		})
		assertFunctionCallsContainAll(t, imageCalls, []string{
			"BuildFinalReviewImages",
		})

		skusSource := readNamedFunctionSource(t, "preview_builder_shein_final_review_skus.go", "buildSheinFinalReviewSKUs")
		skusCalls := readNamedFunctionCallNames(t, "preview_builder_shein_final_review_skus.go", "buildSheinFinalReviewSKUs")
		skusFileSource, err := os.ReadFile("preview_builder_shein_final_review_skus.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein_final_review_skus.go) error = %v", err)
		}
		assertSourceContainsAll(t, skusSource, []string{
			"return sheinworkspace.BuildFinalReviewSKUs(draft)",
		})
		if !strings.Contains(string(skusFileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("preview_builder_shein_final_review_skus.go should call marketplace SHEIN workspace directly")
		}
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
