package productimage

import "encoding/json"

type sellingPointFillInput struct {
	VisualMode         string                            `json:"visual_mode,omitempty"`
	LayoutVariant      string                            `json:"layout_variant,omitempty"`
	BackgroundTemplate string                            `json:"background_template,omitempty"`
	OverlayTemplate    string                            `json:"overlay_template,omitempty"`
	EngineVersion      string                            `json:"layout_engine_version,omitempty"`
	SlotPlanVersion    string                            `json:"slot_plan_version,omitempty"`
	ContentPlanVersion string                            `json:"content_plan_version,omitempty"`
	Slots              *sellingPointFillInputSlots       `json:"slots,omitempty"`
	Constraints        *sellingPointFillInputConstraints `json:"constraints,omitempty"`
	Content            *sellingPointFillInputContent     `json:"content,omitempty"`
}

type sellingPointFillInputSlots struct {
	Copy          []string `json:"copy,omitempty"`
	Badges        []string `json:"badges,omitempty"`
	Measurements  []string `json:"measurements,omitempty"`
	DetailAnchors []string `json:"detail_anchors,omitempty"`
}

type sellingPointFillInputConstraints struct {
	MaxCopyLines     int    `json:"max_copy_lines,omitempty"`
	MaxBadges        int    `json:"max_badges,omitempty"`
	MeasurementMode  string `json:"measurement_mode,omitempty"`
	DetailAnchorMode string `json:"detail_anchor_mode,omitempty"`
}

type sellingPointFillInputContent struct {
	Copy          []sellingPointContentEntry `json:"copy,omitempty"`
	Badges        []sellingPointContentEntry `json:"badges,omitempty"`
	Measurements  []sellingPointContentEntry `json:"measurements,omitempty"`
	DetailAnchors []sellingPointContentEntry `json:"detail_anchors,omitempty"`
}

func buildSellingPointFillInput(profile sceneProfile, productContext *ProductContext) *sellingPointFillInput {
	slotPlan := buildSellingPointSlotPlan(profile)
	if slotPlan == nil {
		return nil
	}
	contentPlan := buildSellingPointContentPlan(profile, productContext)
	return &sellingPointFillInput{
		VisualMode:         profile.visualMode,
		LayoutVariant:      profile.layoutVariant,
		BackgroundTemplate: profile.backgroundTemplate,
		OverlayTemplate:    profile.overlayTemplate,
		EngineVersion:      "v1",
		SlotPlanVersion:    "v1",
		ContentPlanVersion: "v1",
		Slots: &sellingPointFillInputSlots{
			Copy:          append([]string(nil), slotPlan.CopySlots...),
			Badges:        append([]string(nil), slotPlan.BadgeSlots...),
			Measurements:  append([]string(nil), slotPlan.MeasurementSlots...),
			DetailAnchors: append([]string(nil), slotPlan.DetailAnchors...),
		},
		Constraints: &sellingPointFillInputConstraints{
			MaxCopyLines:     slotPlan.MaxCopyLines,
			MaxBadges:        slotPlan.MaxBadges,
			MeasurementMode:  slotPlan.MeasurementMode,
			DetailAnchorMode: slotPlan.DetailAnchorMode,
		},
		Content: cloneSellingPointContentPlan(contentPlan),
	}
}

func cloneSellingPointContentPlan(plan *sellingPointContentPlan) *sellingPointFillInputContent {
	if plan == nil {
		return nil
	}
	return &sellingPointFillInputContent{
		Copy:          append([]sellingPointContentEntry(nil), plan.Copy...),
		Badges:        append([]sellingPointContentEntry(nil), plan.Badges...),
		Measurements:  append([]sellingPointContentEntry(nil), plan.Measurements...),
		DetailAnchors: append([]sellingPointContentEntry(nil), plan.DetailAnchors...),
	}
}

func applySellingPointFillInputMetadata(metadata map[string]string, profile sceneProfile, productContext *ProductContext) {
	if metadata == nil {
		return
	}
	input := buildSellingPointFillInput(profile, productContext)
	if input == nil {
		return
	}
	data, err := json.Marshal(input)
	if err != nil {
		return
	}
	setMetadataDefault(metadata, "layout_fill_input", string(data))
	setMetadataDefault(metadata, "layout_fill_input_version", "v1")
}

func ApplySellingPointFillInputMetadata(metadata map[string]string, profileName string, productContext *ProductContext) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	applySellingPointFillInputMetadata(metadata, profile, productContext)
	return metadata
}
