package productimage

import (
	"encoding/json"
	"testing"
)

func TestApplySellingPointFillInputMetadataAddsUnifiedFillInput(t *testing.T) {
	t.Parallel()

	metadata := ApplySellingPointFillInputMetadata(map[string]string{}, "shein_selling_point", &ProductContext{
		Title:       "Portable Speaker",
		ProductType: "Bluetooth Speaker",
		Attributes: map[string]string{
			"Material": "ABS",
			"Size":     "12 x 8 x 3 cm",
			"Feature":  "Water Resistant",
		},
	})

	if metadata["layout_fill_input"] == "" || metadata["layout_fill_input_version"] != "v1" {
		t.Fatalf("metadata = %+v, want unified fill input metadata", metadata)
	}

	var input sellingPointFillInput
	if err := json.Unmarshal([]byte(metadata["layout_fill_input"]), &input); err != nil {
		t.Fatalf("unmarshal fill input: %v", err)
	}
	if input.VisualMode != "selling_point" || input.LayoutVariant == "" || input.BackgroundTemplate == "" || input.OverlayTemplate == "" {
		t.Fatalf("fill input = %+v, want preset resource fields", input)
	}
	if input.Slots == nil || len(input.Slots.Copy) == 0 || len(input.Slots.Badges) == 0 {
		t.Fatalf("fill input = %+v, want slot plan", input)
	}
	if input.Constraints == nil || input.Constraints.MaxCopyLines == 0 || input.Constraints.MaxBadges == 0 {
		t.Fatalf("fill input = %+v, want constraints", input)
	}
	if input.Content == nil || len(input.Content.Copy) == 0 || input.Content.Copy[0].SourceKey == "" || input.Content.Copy[0].ContentType == "" {
		t.Fatalf("fill input = %+v, want structured content entries", input)
	}
}
