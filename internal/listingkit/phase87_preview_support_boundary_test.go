package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinPreviewSupportBoundary(t *testing.T) {
	t.Parallel()

	t.Run("resolution cache summary delegates to workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_builder_shein_resolution_cache.go", "buildSheinResolutionCacheSummary")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein_resolution_cache.go", "buildSheinResolutionCacheSummary")
		fileSource, err := os.ReadFile("preview_builder_shein_resolution_cache.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein_resolution_cache.go) error = %v", err)
		}

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.BuildResolutionCacheSummary(pkg)",
		})
		if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("preview_builder_shein_resolution_cache.go should call marketplace SHEIN workspace directly")
		}
		assertSourceExcludesAll(t, source, []string{
			"enrichCategoryResolutionCacheInfo(",
			"enrichPricingResolutionCacheInfo(",
			"CloneResolutionCacheInfo(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"BuildResolutionCacheSummary",
		})
	})

	t.Run("image upload preflight delegates aggregation to workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "preview_builder_shein_image_upload.go", "buildSheinImageUploadPreflight")
		callNames := readNamedFunctionCallNames(t, "preview_builder_shein_image_upload.go", "buildSheinImageUploadPreflight")
		fileSource, err := os.ReadFile("preview_builder_shein_image_upload.go")
		if err != nil {
			t.Fatalf("ReadFile(preview_builder_shein_image_upload.go) error = %v", err)
		}

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.BuildImageUploadPreflight(",
			"sheinpub.IsUploadedImageURL,",
			"sheinImageUploadCache(pkg)[strings.TrimSpace(sourceURL)]",
			"sheinpub.IsSDSImageURL,",
		})
		if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("preview_builder_shein_image_upload.go should call marketplace SHEIN workspace directly")
		}
		assertSourceExcludesAll(t, source, []string{
			"collectSheinProductImageURLs(pkg.PreviewPayload)",
			"buildSheinImageUploadPreflightSummary(report)",
			"report.PendingUploadURLs++",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"BuildImageUploadPreflight",
		})
	})
}
