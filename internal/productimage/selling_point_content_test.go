package productimage

import (
	"strings"
	"testing"
)

func TestApplySellingPointContentPlanMetadataAddsContentPlan(t *testing.T) {
	t.Parallel()

	profile := defaultSceneProfile("shein_selling_point")
	profile.visualMode = "selling_point"
	profile.copySlots = []string{"headline", "subhead"}
	profile.badgeSlots = []string{"badge_top_left"}
	profile.measurementSlots = []string{"measurement_bottom"}
	profile.detailAnchorSlots = []string{"detail_right"}
	profile.maxCopyLines = 2
	profile.maxBadges = 1

	metadata := map[string]string{}
	applySellingPointContentPlanMetadata(metadata, profile, &ProductContext{
		Title:       "Portable Speaker",
		ProductType: "Bluetooth Speaker",
		Attributes: map[string]string{
			"Material": "ABS",
			"Size":     "12 x 8 x 3 cm",
			"Feature":  "Water Resistant",
		},
	})

	if !strings.Contains(metadata["layout_content.copy"], "Portable Speaker") {
		t.Fatalf("metadata = %+v, want copy content", metadata)
	}
	if !strings.Contains(metadata["layout_content.copy"], "\"source_key\":\"title\"") || !strings.Contains(metadata["layout_content.copy"], "\"content_type\":\"headline\"") {
		t.Fatalf("metadata = %+v, want structured copy source metadata", metadata)
	}
	if !strings.Contains(metadata["layout_content.badges"], "Feature: Water Resistant") && !strings.Contains(metadata["layout_content.badges"], "Material: ABS") {
		t.Fatalf("metadata = %+v, want badge content", metadata)
	}
	if !strings.Contains(metadata["layout_content.badges"], "\"source_type\":\"attribute\"") {
		t.Fatalf("metadata = %+v, want structured badge source metadata", metadata)
	}
	if !strings.Contains(metadata["layout_content.measurements"], "Size: 12 x 8 x 3 cm") {
		t.Fatalf("metadata = %+v, want measurement content", metadata)
	}
	if metadata["content_plan_version"] != "v1" {
		t.Fatalf("metadata = %+v, want content plan version", metadata)
	}
}
