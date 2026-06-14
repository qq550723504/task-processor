package preview

import "testing"

type renderSummarySlot struct {
	visualMode   string
	capabilities []string
}

func TestSummarizePlatformRenderPreviewsCountsSlotsCapabilitiesAndVisualModes(t *testing.T) {
	main := renderSummarySlot{
		visualMode:   "hero",
		capabilities: []string{"subject_preview", "text_preview"},
	}

	got := SummarizePlatformRenderPreviews(PlatformRenderPreviewSummaryInput[renderSummarySlot]{
		Main: &main,
		Gallery: []renderSummarySlot{
			{visualMode: "gallery", capabilities: []string{"subject_preview"}},
			{visualMode: "gallery", capabilities: []string{"subject_preview", "style_preview"}},
		},
		Auxiliary: []renderSummarySlot{
			{capabilities: []string{"debug_preview"}},
		},
		VisualMode: func(slot renderSummarySlot) string {
			return slot.visualMode
		},
		Capabilities: func(slot renderSummarySlot) []string {
			return slot.capabilities
		},
	})

	if got == nil {
		t.Fatal("summary = nil, want summary")
	}
	if got.TotalPreviews != 4 || !got.MainAvailable || got.GalleryCount != 2 || got.AuxiliaryCount != 1 {
		t.Fatalf("summary counts = %+v, want total/main/gallery/auxiliary counts", got)
	}
	if got.CapabilityCounts["subject_preview"] != 3 || got.CapabilityCounts["text_preview"] != 1 || got.CapabilityCounts["style_preview"] != 1 || got.CapabilityCounts["debug_preview"] != 1 {
		t.Fatalf("CapabilityCounts = %+v, want counted capabilities", got.CapabilityCounts)
	}
	if want := []string{"hero", "gallery"}; !equalStrings(got.VisualModes, want) {
		t.Fatalf("VisualModes = %+v, want %+v", got.VisualModes, want)
	}
}

func TestSummarizePlatformRenderPreviewsReturnsNilForEmptyInput(t *testing.T) {
	got := SummarizePlatformRenderPreviews(PlatformRenderPreviewSummaryInput[renderSummarySlot]{})

	if got != nil {
		t.Fatalf("summary = %+v, want nil", got)
	}
}
