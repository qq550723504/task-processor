package generation

import (
	"reflect"
	"testing"
)

func TestRenderPreviewCapabilitiesMapsKnownLayerTypes(t *testing.T) {
	t.Parallel()

	got := RenderPreviewCapabilities([]string{
		"badge",
		"badge",
		"spec",
		"unknown",
		"subject",
	})
	want := []string{"badge_preview", "measurement_preview", "subject_preview"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderPreviewCapabilities() = %+v, want %+v", got, want)
	}
}

func TestRenderPreviewCapabilitiesForSlotFallsBackToSubjectForRasterAsset(t *testing.T) {
	t.Parallel()

	got := RenderPreviewCapabilitiesForSlot(nil, "", "http://127.0.0.1:9100/listingkit-assets/gallery-1.png")
	want := []string{"subject_preview"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RenderPreviewCapabilitiesForSlot() = %+v, want %+v", got, want)
	}
}

func TestRenderPreviewCapabilitiesForSlotSkipsFallbackForSVGOrMissingAsset(t *testing.T) {
	t.Parallel()

	if got := RenderPreviewCapabilitiesForSlot(nil, "<svg />", "http://127.0.0.1:9100/listingkit-assets/gallery-1.png"); got != nil {
		t.Fatalf("RenderPreviewCapabilitiesForSlot() with SVG = %+v, want nil", got)
	}
	if got := RenderPreviewCapabilitiesForSlot(nil, "", ""); got != nil {
		t.Fatalf("RenderPreviewCapabilitiesForSlot() without asset = %+v, want nil", got)
	}
}
