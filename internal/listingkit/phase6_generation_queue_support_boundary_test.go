package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationQueueSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("generation_queue.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_queue.go) error = %v", err)
	}
	bundleSrc, err := os.ReadFile("generation_queue_bundle_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_queue_bundle_support.go) error = %v", err)
	}
	summarySrc, err := os.ReadFile("generation_queue_summary_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_queue_summary_support.go) error = %v", err)
	}

	rootContent := string(rootSrc)
	bundleContent := string(bundleSrc)
	summaryContent := string(summarySrc)

	for _, needle := range []string{
		"func buildGenerationWorkQueue(result *ListingKitResult) *GenerationWorkQueue {",
		"func indexGenerationWorkQueue(queue *GenerationWorkQueue) map[generationQueueKey]GenerationWorkQueueItem {",
		"func indexAssetRenderPreviews(result *ListingKitResult) map[string]AssetRenderPreview {",
		"func indexGenerationScenePresets(result *ListingKitResult) map[string]*GenerationScenePresetSummary {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("generation_queue.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func generationQueueBundles(result *ListingKitResult) []struct {",
		"func appendBundleQueueItems(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, renderPreviewIndex map[string]AssetRenderPreview, scenePresetIndex map[string]*GenerationScenePresetSummary, platform string, bundle *common.PublishImageBundle) {",
		"func appendMissingSlotQueueItem(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, platform string, slot common.MissingSlot) {",
		"func generationQueueSlotExecutionQuality(slot common.BundleSlot) string {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("generation_queue.go should delegate bundle helper %q", needle)
		}
		if !strings.Contains(bundleContent, needle) {
			t.Fatalf("generation_queue_bundle_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func generationQualityGrade(value string) string {",
		"func generationExecutionQualityLabel(value string) string {",
		"func buildGenerationWorkQueueSummary(items []GenerationWorkQueueItem) *GenerationWorkQueueSummary {",
		"func accumulateGenerationQueueQualityMetrics(summary *GenerationWorkQueueSummary, item GenerationWorkQueueItem) {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("generation_queue.go should delegate summary helper %q", needle)
		}
		if !strings.Contains(summaryContent, needle) {
			t.Fatalf("generation_queue_summary_support.go should contain %q", needle)
		}
	}
}
