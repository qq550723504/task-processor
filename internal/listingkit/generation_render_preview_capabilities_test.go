package listingkit

import "testing"

func TestBuildRenderPreviewCapabilitiesForSlotFallsBackToSubjectForRasterPreview(t *testing.T) {
	t.Parallel()

	capabilities := buildRenderPreviewCapabilitiesForSlot(AssetRenderPreviewSlot{
		AssetURL: "http://127.0.0.1:9100/listingkit-assets/gallery-1.png",
	})
	if len(capabilities) != 1 || capabilities[0] != "subject_preview" {
		t.Fatalf("capabilities = %+v, want subject_preview fallback", capabilities)
	}
}
