package productimage

import (
	"encoding/json"
	"testing"
)

func TestApplySellingPointRenderBlocksMetadataAddsRenderableBlocks(t *testing.T) {
	t.Parallel()

	metadata := ApplySellingPointRenderBlocksMetadata(map[string]string{}, "shein_selling_point", &ProductContext{
		Title:       "Portable Speaker",
		ProductType: "Bluetooth Speaker",
		Attributes: map[string]string{
			"Material": "ABS",
			"Size":     "12 x 8 x 3 cm",
			"Feature":  "Water Resistant",
		},
	})

	if metadata["layout_render_blocks"] == "" || metadata["render_block_plan_version"] != "v1" {
		t.Fatalf("metadata = %+v, want render blocks metadata", metadata)
	}

	var blocks []sellingPointRenderBlock
	if err := json.Unmarshal([]byte(metadata["layout_render_blocks"]), &blocks); err != nil {
		t.Fatalf("unmarshal render blocks: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatalf("blocks = %+v, want renderable blocks", blocks)
	}
	if blocks[0].ID == "" || blocks[0].Kind == "" || blocks[0].Slot == "" || blocks[0].Text == "" {
		t.Fatalf("blocks = %+v, want complete render block fields", blocks)
	}
	if blocks[0].Priority == 0 {
		t.Fatalf("blocks = %+v, want block priority", blocks)
	}
}
