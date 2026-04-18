package listingkit

import (
	"testing"

	"task-processor/internal/asset"
)

func TestBuildActionPlatformRenderPreviewsFiltersByPlatformAndCapability(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{
			{
				Platform: "shein",
				Main: &AssetRenderPreviewSlot{
					Slot:       "main",
					AssetID:    "asset-1",
					PreviewSVG: "<svg/>",
					LayerTypes: []string{"background", "detail", "text"},
				},
			},
			{
				Platform: "amazon",
				Main: &AssetRenderPreviewSlot{
					Slot:       "main",
					AssetID:    "asset-2",
					PreviewSVG: "<svg/>",
					LayerTypes: []string{"background", "badge", "text"},
				},
			},
		},
		AssetBundle: &asset.Bundle{},
	}

	previews := buildActionPlatformRenderPreviews(result, &GenerationQueueQuery{
		Platform:                      "shein",
		PreviewCapability:             "detail_preview",
		RenderPreviewAvailable:        true,
		RenderPreviewAvailablePresent: true,
	})
	if len(previews) != 1 {
		t.Fatalf("previews = %+v, want one filtered platform", previews)
	}
	if previews[0].Platform != "shein" {
		t.Fatalf("preview platform = %+v, want shein", previews[0])
	}
	if previews[0].Main == nil || previews[0].Main.AssetID != "asset-1" {
		t.Fatalf("preview main = %+v, want shein main preview", previews[0].Main)
	}
	if previews[0].Summary == nil || previews[0].Summary.CapabilityCounts["detail_preview"] != 1 {
		t.Fatalf("preview summary = %+v, want detail capability summary", previews[0].Summary)
	}
}
