package productimage

import (
	"encoding/json"
	"testing"
)

func TestApplySellingPointDrawOutputMetadataAddsDeterministicDrawOutput(t *testing.T) {
	t.Parallel()

	metadata := ApplySellingPointDrawOutputMetadata(map[string]string{}, "shein_selling_point", &ProductContext{
		Title:       "Portable Speaker",
		ProductType: "Bluetooth Speaker",
		Attributes: map[string]string{
			"Material": "ABS",
			"Size":     "12 x 8 x 3 cm",
			"Feature":  "Water Resistant",
		},
	})

	if metadata["layout_draw_output"] == "" || metadata["draw_output_version"] != "v1" {
		t.Fatalf("metadata = %+v, want draw output metadata", metadata)
	}

	var output sellingPointDrawOutput
	if err := json.Unmarshal([]byte(metadata["layout_draw_output"]), &output); err != nil {
		t.Fatalf("unmarshal draw output: %v", err)
	}
	if output.Renderer == "" || output.VisualMode != "selling_point" {
		t.Fatalf("output = %+v, want draw output identity", output)
	}
	if len(output.Instructions) == 0 {
		t.Fatalf("output = %+v, want draw instructions", output)
	}

	first := output.Instructions[0]
	if first.ID == "" || first.LayerType == "" || first.Region == "" || first.RenderOrder == 0 || first.ZIndex == 0 {
		t.Fatalf("output = %+v, want structured draw instruction", output)
	}
	if first.Bounds == nil || first.Bounds.Width <= 0 || first.Bounds.Height <= 0 {
		t.Fatalf("output = %+v, want normalized draw bounds", output)
	}
	if first.Opacity <= 0 {
		t.Fatalf("output = %+v, want opacity", output)
	}

	foundText := false
	foundBadge := false
	for _, instruction := range output.Instructions {
		if instruction.LayerType == "text" {
			foundText = true
			if instruction.TextStyle == "" || instruction.Text == "" {
				t.Fatalf("output = %+v, want text instruction payload", output)
			}
		}
		if instruction.LayerType == "badge" {
			foundBadge = true
			if instruction.BadgeStyle == "" {
				t.Fatalf("output = %+v, want badge instruction payload", output)
			}
		}
	}
	if !foundText || !foundBadge {
		t.Fatalf("output = %+v, want text and badge draw instructions", output)
	}
}
