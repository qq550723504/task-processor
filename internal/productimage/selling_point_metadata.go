package productimage

import "strings"

func applySellingPointSlotPlanMetadata(metadata map[string]string, profile sceneProfile) {
	if metadata == nil {
		return
	}
	plan := buildSellingPointSlotPlan(profile)
	if plan == nil {
		return
	}
	setMetadataDefault(metadata, "layout_slots.copy", strings.Join(plan.CopySlots, ","))
	setMetadataDefault(metadata, "layout_slots.badges", strings.Join(plan.BadgeSlots, ","))
	setMetadataDefault(metadata, "layout_slots.measurements", strings.Join(plan.MeasurementSlots, ","))
	setMetadataDefault(metadata, "layout_slots.detail_anchors", strings.Join(plan.DetailAnchors, ","))
	setMetadataDefault(metadata, "layout_constraints.max_copy_lines", intToString(plan.MaxCopyLines))
	setMetadataDefault(metadata, "layout_constraints.max_badges", intToString(plan.MaxBadges))
	setMetadataDefault(metadata, "layout_constraints.measurement_mode", plan.MeasurementMode)
	setMetadataDefault(metadata, "layout_constraints.detail_anchor_mode", plan.DetailAnchorMode)
	setMetadataDefault(metadata, "layout_engine_version", "v1")
	setMetadataDefault(metadata, "slot_plan_version", "v1")
}

func ApplySellingPointContentPlanMetadata(metadata map[string]string, profileName string, productContext *ProductContext) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	applySellingPointContentPlanMetadata(metadata, profile, productContext)
	return metadata
}
