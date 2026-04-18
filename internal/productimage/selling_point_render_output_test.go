package productimage

import (
	"encoding/json"
	"testing"
)

func TestApplySellingPointRenderOutputMetadataAddsRenderableOutput(t *testing.T) {
	t.Parallel()

	metadata := ApplySellingPointRenderOutputMetadata(map[string]string{}, "shein_selling_point", &ProductContext{
		Title:       "Portable Speaker",
		ProductType: "Bluetooth Speaker",
		Attributes: map[string]string{
			"Material": "ABS",
			"Size":     "12 x 8 x 3 cm",
			"Feature":  "Water Resistant",
		},
	})

	if metadata["layout_render_output"] == "" || metadata["render_output_version"] != "v2" {
		t.Fatalf("metadata = %+v, want render output metadata", metadata)
	}

	var output sellingPointRenderOutput
	if err := json.Unmarshal([]byte(metadata["layout_render_output"]), &output); err != nil {
		t.Fatalf("unmarshal render output: %v", err)
	}
	if output.LayoutEngine == "" || output.VisualMode != "selling_point" {
		t.Fatalf("output = %+v, want selling-point output identity", output)
	}
	if len(output.Layers) == 0 {
		t.Fatalf("output = %+v, want render output layers", output)
	}
	first := output.Layers[0]
	if first.ID == "" || first.Layer == "" || first.Region == "" || first.RenderOrder == 0 {
		t.Fatalf("output = %+v, want structured render output layer", output)
	}
	if first.LayerType == "" || first.StyleToken == "" || first.Alignment == "" || first.ZIndex == 0 {
		t.Fatalf("output = %+v, want explicit render parameters", output)
	}
	if first.Bounds == nil || first.Bounds.Width <= 0 || first.Bounds.Height <= 0 {
		t.Fatalf("output = %+v, want normalized bounds", output)
	}
	foundTextLayer := false
	foundBadgeLayer := false
	for _, layer := range output.Layers {
		if layer.LayerType == "text" {
			foundTextLayer = true
			if layer.TextStyle == "" {
				t.Fatalf("output = %+v, want text style for text layer", output)
			}
		}
		if layer.LayerType == "badge" {
			foundBadgeLayer = true
			if layer.BadgeStyle == "" {
				t.Fatalf("output = %+v, want badge style for badge layer", output)
			}
		}
	}
	if !foundTextLayer || !foundBadgeLayer {
		t.Fatalf("output = %+v, want text and badge layers", output)
	}
}
