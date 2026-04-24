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

func TestBuildActionPlatformRenderPreviewsKeepsRasterGalleryForSubjectPreview(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		PlatformAssetRenderPreviews: []PlatformAssetRenderPreviews{
			{
				Platform: "shein",
				Gallery: []AssetRenderPreviewSlot{
					{
						Slot:     "gallery",
						AssetID:  "asset-gallery-1",
						AssetURL: "http://127.0.0.1:9100/listingkit-assets/gallery-1.png",
					},
				},
			},
		},
		AssetBundle: &asset.Bundle{},
	}

	previews := buildActionPlatformRenderPreviews(result, &GenerationQueueQuery{
		Platform:                      "shein",
		Slot:                          "gallery",
		PreviewCapability:             "subject_preview",
		RenderPreviewAvailable:        true,
		RenderPreviewAvailablePresent: true,
	})
	if len(previews) != 1 {
		t.Fatalf("previews = %+v, want one filtered platform", previews)
	}
	if len(previews[0].Gallery) != 1 || previews[0].Gallery[0].AssetID != "asset-gallery-1" {
		t.Fatalf("gallery previews = %+v, want raster gallery retained for subject preview", previews[0].Gallery)
	}
}
