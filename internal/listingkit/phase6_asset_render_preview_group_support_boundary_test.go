package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestAssetRenderPreviewGroupSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("asset_render_preview_groups.go")
	if err != nil {
		t.Fatalf("ReadFile(asset_render_preview_groups.go) error = %v", err)
	}
	slotSrc, err := os.ReadFile("asset_render_preview_group_slot_support.go")
	if err != nil {
		t.Fatalf("ReadFile(asset_render_preview_group_slot_support.go) error = %v", err)
	}
	urlSrc, err := os.ReadFile("asset_render_preview_group_url_support.go")
	if err != nil {
		t.Fatalf("ReadFile(asset_render_preview_group_url_support.go) error = %v", err)
	}

	rootContent := string(rootSrc)
	slotContent := string(slotSrc)
	urlContent := string(urlSrc)

	for _, needle := range []string{
		"func buildPlatformAssetRenderPreviewSummary(group PlatformAssetRenderPreviews) *PlatformAssetRenderPreviewSummary {",
		"func buildPlatformAssetRenderPreviews(result *ListingKitResult) []PlatformAssetRenderPreviews {",
		"func syncAssetRenderPreviews(result *ListingKitResult) {",
		"func filterPlatformAssetRenderPreviews(groups []PlatformAssetRenderPreviews, platform string) []PlatformAssetRenderPreviews {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("asset_render_preview_groups.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildPlatformAssetRenderPreviewGroup(platform string, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle, bundle *common.PublishImageBundle) (PlatformAssetRenderPreviews, bool) {",
		"func buildAssetRenderPreviewSlots(defaultSlot string, slots []common.BundleSlot, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle) []AssetRenderPreviewSlot {",
		"func buildAssetRenderPreviewSlot(defaultSlot string, slot *common.BundleSlot, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle) *AssetRenderPreviewSlot {",
		"func imageBundleFromShein(pkg *SheinPackage) *common.PublishImageBundle {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("asset_render_preview_groups.go should delegate slot helper %q", needle)
		}
		if !strings.Contains(slotContent, needle) {
			t.Fatalf("asset_render_preview_group_slot_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildAssetURLLookup(bundle *asset.Bundle) map[string]string {",
		"func publishedAssetURLForBundleSlot(slot *common.BundleSlot, assetURLByID map[string]string, assetBundle *asset.Bundle) string {",
		"func matchesPublishedBundleAssetForSlot(item asset.Asset, slot *common.BundleSlot) bool {",
		"func isPublishedAssetURL(value string) bool {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("asset_render_preview_groups.go should delegate url helper %q", needle)
		}
		if !strings.Contains(urlContent, needle) {
			t.Fatalf("asset_render_preview_group_url_support.go should contain %q", needle)
		}
	}
}
