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

		source := readNamedFunctionSource(t, "preview_builder_shein_final_review.go", "buildSheinFinalReviewPayload")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein_final_review.go", "buildSheinFinalReviewPayload")
		fileSource, err := os.ReadFile("preview_builder_shein_final_review.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein_final_review.go) error = %v", err)
		}
		assertSourceContainsAll(t, source, []string{
			"final.SKUs = sheinworkspace.BuildFinalReviewSKUs(pkg.DraftPayload)",
			"final.Images = sheinworkspace.BuildFinalReviewImages(pkg.DraftPayload, pkg.FinalSubmissionDraft, pkg.PreviewPayload)",
		})
		if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("preview_builder_shein_final_review.go should call marketplace SHEIN workspace directly")
		}
		assertSourceExcludesAll(t, source, []string{
			"sheinproduct.CollectSizeMapImageURLs(product)",
			"mergeSheinFinalReviewImage(",
			"for _, skc := range draft.SKCList",
			"buildSheinFinalReviewSKU(skc.SupplierCode, sku)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"BuildFinalReviewSKUs",
			"BuildFinalReviewImages",
		})
		assertFileAbsent(t, "preview_builder_shein_final_review_images.go")
		assertFileAbsent(t, "preview_builder_shein_final_review_skus.go")
	})
}
