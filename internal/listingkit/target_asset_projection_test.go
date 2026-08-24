package listingkit

import (
	"testing"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/listingkit/core"
	common "task-processor/internal/publishing/common"
)

func TestSelectedPlatformPreviewAndExportUseTargetKeyedAssetsWithReversedTargets(t *testing.T) {
	t.Parallel()

	temuBundle := targetAssetBundle("temu-main", "https://cdn.example.test/temu-main.jpg")
	sheinBundle := targetAssetBundle("shein-main", "https://cdn.example.test/shein-main.jpg")
	result := &ListingKitResult{
		Platforms:             []string{"temu", "shein"},
		AssetBundle:           temuBundle,
		AssetInventorySummary: &asset.InventorySummary{TotalRecords: 2},
		AssetBundlesByTarget: map[string]*asset.Bundle{
			"temu":  temuBundle,
			"shein": sheinBundle,
		},
		AssetInventorySummariesByTarget: map[string]*asset.InventorySummary{
			"temu":  {TotalRecords: 2},
			"shein": {TotalRecords: 11},
		},
		Shein: &SheinPackage{ImageBundle: &common.PublishImageBundle{
			Platform: "shein",
			Main: &common.BundleSlot{
				AssetID: "shein-main",
				URL:     "https://cdn.example.test/shein-main.jpg",
			},
		}},
		Temu: &TemuPackage{ImageBundle: &common.PublishImageBundle{
			Platform: "temu",
			Main: &common.BundleSlot{
				AssetID: "temu-main",
				URL:     "https://cdn.example.test/temu-main.jpg",
			},
		}},
	}
	task := &Task{
		ID:        "targeted-preview-export",
		Status:    core.TaskStatusCompleted,
		CreatedAt: time.Now().Add(-time.Minute),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"temu", "shein"}},
		Result:    result,
	}

	preview, err := buildListingKitPreview(task, "shein")
	if err != nil {
		t.Fatalf("buildListingKitPreview() error = %v", err)
	}
	assertTargetedAssetProjection(t, preview.Assets, preview.AssetInventory, preview.AssetRenderPreviews, preview.PlatformAssetRenderPreviews)

	export, err := buildListingKitExport(task, "shein")
	if err != nil {
		t.Fatalf("buildListingKitExport() error = %v", err)
	}
	assertTargetedAssetProjection(t, export.AssetBundle, export.AssetInventorySummary, export.AssetRenderPreviews, export.PlatformAssetRenderPreviews)
}

func targetAssetBundle(id, url string) *asset.Bundle {
	return &asset.Bundle{Assets: []asset.Asset{{
		ID:   id,
		Kind: asset.KindMainImage,
		URL:  url,
		Metadata: map[string]string{
			"layout_draw_preview_svg": "<svg>" + id + "</svg>",
			"draw_preview_format":     "svg",
		},
	}}}
}

func assertTargetedAssetProjection(t *testing.T, bundle *asset.Bundle, summary *asset.InventorySummary, previews []AssetRenderPreview, groups []PlatformAssetRenderPreviews) {
	t.Helper()
	if bundle == nil || len(bundle.Assets) != 1 || bundle.Assets[0].ID != "shein-main" {
		t.Fatalf("asset bundle = %#v, want shein target bundle", bundle)
	}
	if summary == nil || summary.TotalRecords != 11 {
		t.Fatalf("asset inventory summary = %#v, want shein target summary", summary)
	}
	if len(previews) != 1 || previews[0].AssetID != "shein-main" {
		t.Fatalf("asset render previews = %#v, want shein target preview", previews)
	}
	if len(groups) != 1 || groups[0].Platform != "shein" || groups[0].Main == nil || groups[0].Main.AssetURL != "https://cdn.example.test/shein-main.jpg" {
		t.Fatalf("platform render previews = %#v, want shein target preview", groups)
	}
}
