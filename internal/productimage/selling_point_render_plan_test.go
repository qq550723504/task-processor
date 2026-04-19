package productimage

import (
	"encoding/json"
	"testing"
)

func TestApplySellingPointRenderPlanMetadataAddsPlan(t *testing.T) {
	t.Parallel()

	metadata := ApplySellingPointRenderPlanMetadata(map[string]string{}, "shein_selling_point", &ProductContext{
		Title:       "Portable Speaker",
		ProductType: "Bluetooth Speaker",
		Attributes: map[string]string{
			"Material": "ABS",
			"Size":     "12 x 8 x 3 cm",
			"Feature":  "Water Resistant",
		},
	})

	if metadata["layout_render_plan"] == "" || metadata["render_plan_version"] != "v1" {
		t.Fatalf("metadata = %+v, want render plan metadata", metadata)
	}

	var plan sellingPointRenderPlan
	if err := json.Unmarshal([]byte(metadata["layout_render_plan"]), &plan); err != nil {
		t.Fatalf("unmarshal render plan: %v", err)
	}
	if plan.VisualMode != "selling_point" || plan.LayoutVariant == "" {
		t.Fatalf("plan = %+v, want selling-point plan identity", plan)
	}
	if len(plan.Items) == 0 {
		t.Fatalf("plan = %+v, want render plan items", plan)
	}
	if plan.Items[0].BlockID == "" || plan.Items[0].Region == "" || plan.Items[0].VisualRole == "" || plan.Items[0].RenderOrder == 0 {
		t.Fatalf("plan = %+v, want structured render plan item", plan)
	}
}
