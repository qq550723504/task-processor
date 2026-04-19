package listingkit

import (
	"testing"

	"task-processor/internal/asset"
)

func TestBuildAssetRenderPreviewsIncludesRenderSummary(t *testing.T) {
	t.Parallel()

	bundle := &asset.Bundle{
		Assets: []asset.Asset{
			{
				ID:   "asset-selling-point",
				Kind: asset.KindSellingPointImage,
				Role: "selling_point",
				Metadata: map[string]string{
					"render_profile":          "shein_selling_point",
					"template_label":          "SHEIN Selling Point",
					"draw_preview_format":     "svg",
					"layout_draw_preview_svg": "<svg/>",
					"render_output_version":   "v2",
					"draw_output_version":     "v1",
					"draw_preview_version":    "v1",
					"layout_render_output": `{
						"layout_engine":"selling_point_output_v2",
						"visual_mode":"selling_point",
						"layers":[
							{"layer_type":"background","region":"full_canvas","style_token":"bg-soft"},
							{"layer_type":"badge","region":"title_band","style_token":"badge-dark"},
							{"layer_type":"text","region":"body_copy","style_token":"copy-primary"}
						]
					}`,
				},
			},
		},
	}

	previews := buildAssetRenderPreviews(bundle)
	if len(previews) != 1 {
		t.Fatalf("previews = %+v", previews)
	}
	preview := previews[0]
	if preview.VisualMode != "selling_point" {
		t.Fatalf("visual_mode = %q, want selling_point", preview.VisualMode)
	}
	if preview.LayoutEngine != "selling_point_output_v2" {
		t.Fatalf("layout_engine = %q, want selling_point_output_v2", preview.LayoutEngine)
	}
	if preview.RenderOutputVersion != "v2" || preview.DrawOutputVersion != "v1" || preview.DrawPreviewVersion != "v1" {
		t.Fatalf("preview versions = %+v", preview)
	}
	if len(preview.LayerTypes) != 3 || preview.LayerTypes[0] != "background" || preview.LayerTypes[2] != "text" {
		t.Fatalf("layer_types = %+v", preview.LayerTypes)
	}
	if len(preview.Regions) != 3 || preview.Regions[1] != "title_band" {
		t.Fatalf("regions = %+v", preview.Regions)
	}
	if len(preview.StyleTokens) != 3 || preview.StyleTokens[2] != "copy-primary" {
		t.Fatalf("style_tokens = %+v", preview.StyleTokens)
	}
}
