package preview

import "testing"

func TestSummarizeRenderPreviewMetadataIncludesVersionsAndLayerSummary(t *testing.T) {
	got := SummarizeRenderPreviewMetadata(map[string]string{
		"render_output_version": "v2",
		"draw_output_version":   "v1",
		"draw_preview_version":  "v3",
		"layout_render_output": `{
			"layout_engine":"selling_point_output_v2",
			"visual_mode":"selling_point",
			"layers":[
				{"layer_type":"background","region":"full_canvas","style_token":"bg-soft"},
				{"layer_type":"badge","region":"title_band","style_token":"badge-dark"},
				{"layer_type":"badge","region":"title_band","style_token":"badge-dark"},
				{"layer_type":"text","region":"body_copy","style_token":"copy-primary"}
			]
		}`,
	})

	if got.VisualMode != "selling_point" {
		t.Fatalf("VisualMode = %q, want selling_point", got.VisualMode)
	}
	if got.LayoutEngine != "selling_point_output_v2" {
		t.Fatalf("LayoutEngine = %q, want selling_point_output_v2", got.LayoutEngine)
	}
	if got.RenderOutputVersion != "v2" || got.DrawOutputVersion != "v1" || got.DrawPreviewVersion != "v3" {
		t.Fatalf("versions = %+v, want v2/v1/v3", got)
	}
	if want := []string{"background", "badge", "text"}; !equalStrings(got.LayerTypes, want) {
		t.Fatalf("LayerTypes = %+v, want %+v", got.LayerTypes, want)
	}
	if want := []string{"full_canvas", "title_band", "body_copy"}; !equalStrings(got.Regions, want) {
		t.Fatalf("Regions = %+v, want %+v", got.Regions, want)
	}
	if want := []string{"bg-soft", "badge-dark", "copy-primary"}; !equalStrings(got.StyleTokens, want) {
		t.Fatalf("StyleTokens = %+v, want %+v", got.StyleTokens, want)
	}
}

func TestSummarizeRenderPreviewMetadataKeepsVersionsWhenOutputIsInvalid(t *testing.T) {
	got := SummarizeRenderPreviewMetadata(map[string]string{
		"render_output_version": "v2",
		"draw_output_version":   "v1",
		"draw_preview_version":  "v3",
		"layout_render_output":  "{",
	})

	if got.RenderOutputVersion != "v2" || got.DrawOutputVersion != "v1" || got.DrawPreviewVersion != "v3" {
		t.Fatalf("versions = %+v, want preserved versions", got)
	}
	if got.VisualMode != "" || got.LayoutEngine != "" || len(got.LayerTypes) != 0 {
		t.Fatalf("summary = %+v, want only version fields", got)
	}
}

func equalStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
