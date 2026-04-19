package listingkit

type generationReviewSectionConfig struct {
	Capability           string
	ActionKey            string
	Title                string
	Label                string
	Description          string
	EmptyState           string
	DefaultToolbarAction []generationReviewToolbarActionConfig
}

type generationReviewToolbarActionConfig struct {
	Key   string
	Label string
}

func generationReviewSectionConfigs() []generationReviewSectionConfig {
	return []generationReviewSectionConfig{
		{
			Capability:  "detail_preview",
			ActionKey:   assetGenerationActionReviewDetailPreviews,
			Title:       "Detail Review",
			Label:       "Review Detail Previews",
			Description: "Review detail callouts and material-specific emphasis before publish.",
			EmptyState:  "No detail preview is available for the current slot.",
			DefaultToolbarAction: []generationReviewToolbarActionConfig{
				{Key: assetGenerationActionReviewDetailPreviews, Label: "Detail"},
				{Key: "open_preview_svg", Label: "Open SVG"},
			},
		},
		{
			Capability:  "measurement_preview",
			ActionKey:   assetGenerationActionReviewMeasurementPreviews,
			Title:       "Measurement Review",
			Label:       "Review Measurement Previews",
			Description: "Review measurement chips and size annotations for clarity and consistency.",
			EmptyState:  "No measurement preview is available for the current slot.",
			DefaultToolbarAction: []generationReviewToolbarActionConfig{
				{Key: assetGenerationActionReviewMeasurementPreviews, Label: "Measurements"},
				{Key: "open_preview_svg", Label: "Open SVG"},
			},
		},
		{
			Capability:  "badge_preview",
			ActionKey:   assetGenerationActionReviewBadgePreviews,
			Title:       "Badge Review",
			Label:       "Review Badge Previews",
			Description: "Review badge placement, priority, and visual balance.",
			EmptyState:  "No badge preview is available for the current slot.",
			DefaultToolbarAction: []generationReviewToolbarActionConfig{
				{Key: assetGenerationActionReviewBadgePreviews, Label: "Badges"},
				{Key: "open_preview_svg", Label: "Open SVG"},
			},
		},
		{
			Capability:  "copy_preview",
			ActionKey:   assetGenerationActionReviewCopyPreviews,
			Title:       "Copy Review",
			Label:       "Review Copy Previews",
			Description: "Review copy blocks and text density before final publish review.",
			EmptyState:  "No copy preview is available for the current slot.",
			DefaultToolbarAction: []generationReviewToolbarActionConfig{
				{Key: assetGenerationActionReviewCopyPreviews, Label: "Copy"},
				{Key: "open_preview_svg", Label: "Open SVG"},
			},
		},
		{
			Capability:  "subject_preview",
			ActionKey:   assetGenerationActionReviewSubjectPreviews,
			Title:       "Subject Review",
			Label:       "Review Subject Previews",
			Description: "Review subject framing and hero focus for the selected asset.",
			EmptyState:  "No subject preview is available for the current slot.",
			DefaultToolbarAction: []generationReviewToolbarActionConfig{
				{Key: assetGenerationActionReviewSubjectPreviews, Label: "Subject"},
				{Key: "open_preview_svg", Label: "Open SVG"},
			},
		},
	}
}

func generationReviewSectionConfigForCapability(capability string) generationReviewSectionConfig {
	for _, cfg := range generationReviewSectionConfigs() {
		if cfg.Capability == capability {
			return cfg
		}
	}
	return generationReviewSectionConfig{
		Capability:  capability,
		ActionKey:   assetGenerationActionReviewReadyAssets,
		Title:       "Preview Review",
		Label:       "Review Previews",
		Description: "Review the current preview coverage for the selected slot.",
		EmptyState:  "No preview is available for this review section.",
		DefaultToolbarAction: []generationReviewToolbarActionConfig{
			{Key: "open_preview_svg", Label: "Open SVG"},
		},
	}
}

func generationReviewSectionConfigForActionKey(actionKey string) *generationReviewSectionConfig {
	for _, cfg := range generationReviewSectionConfigs() {
		if cfg.ActionKey == actionKey {
			copyCfg := cfg
			return &copyCfg
		}
	}
	return nil
}
