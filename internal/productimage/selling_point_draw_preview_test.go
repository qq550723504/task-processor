package productimage

import (
	"strings"
	"testing"
)

func TestApplySellingPointDrawPreviewMetadataAddsSVGPreview(t *testing.T) {
	t.Parallel()

	metadata := ApplySellingPointDrawPreviewMetadata(map[string]string{}, "shein_selling_point", &ProductContext{
		Title:       "Portable Speaker",
		ProductType: "Bluetooth Speaker",
		Attributes: map[string]string{
			"Color":    "Black",
			"Material": "ABS",
			"Size":     "12 x 8 x 3 cm",
			"Feature":  "Water Resistant",
		},
	})

	if metadata["layout_draw_preview_svg"] == "" || metadata["draw_preview_version"] != "v1" {
		t.Fatalf("metadata = %+v, want draw preview metadata", metadata)
	}
	if metadata["draw_preview_format"] != "svg" {
		t.Fatalf("metadata = %+v, want svg format", metadata)
	}

	svg := metadata["layout_draw_preview_svg"]
	if !strings.Contains(svg, "<svg") || !strings.Contains(svg, "</svg>") {
		t.Fatalf("svg = %q, want svg document", svg)
	}
	if !strings.Contains(svg, "Portable Speaker") {
		t.Fatalf("svg = %q, want selling-point text content", svg)
	}
	if !strings.Contains(svg, "Size: 12 x 8 x 3 cm") {
		t.Fatalf("svg = %q, want measurement content", svg)
	}
	if !strings.Contains(svg, "Color: Black") {
		t.Fatalf("svg = %q, want detail anchor content", svg)
	}
	if !strings.Contains(svg, "selling-point-card") {
		t.Fatalf("svg = %q, want overlay template token", svg)
	}
	if !strings.Contains(svg, `data-layer="measurement-chip"`) {
		t.Fatalf("svg = %q, want dedicated measurement layer", svg)
	}
	if !strings.Contains(svg, `data-layer="detail-callout"`) {
		t.Fatalf("svg = %q, want dedicated detail callout layer", svg)
	}
}
