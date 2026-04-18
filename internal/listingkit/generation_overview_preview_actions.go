package listingkit

type previewCapabilityActionSpec struct {
	Capability string
	ActionKey  string
	Label      string
}

func previewCapabilityActionSpecs() []previewCapabilityActionSpec {
	configs := generationReviewSectionConfigs()
	out := make([]previewCapabilityActionSpec, 0, len(configs))
	for _, cfg := range configs {
		out = append(out, previewCapabilityActionSpec{
			Capability: cfg.Capability,
			ActionKey:  cfg.ActionKey,
			Label:      cfg.Label,
		})
	}
	return out
}

func previewCapabilityActionSpecForKey(actionKey string) *previewCapabilityActionSpec {
	for _, spec := range previewCapabilityActionSpecs() {
		if spec.ActionKey == actionKey {
			copySpec := spec
			return &copySpec
		}
	}
	return nil
}

func buildPreviewCapabilitySecondaryActions(summary *GenerationWorkQueueSummary) ([]string, []string) {
	if summary == nil || len(summary.PreviewCapabilityCounts) == 0 {
		return nil, nil
	}
	actions := make([]string, 0, len(summary.PreviewCapabilityCounts))
	actionKeys := make([]string, 0, len(summary.PreviewCapabilityCounts))
	for _, spec := range previewCapabilityActionSpecs() {
		if summary.PreviewCapabilityCounts[spec.Capability] <= 0 {
			continue
		}
		actions = append(actions, spec.Label)
		actionKeys = append(actionKeys, spec.ActionKey)
	}
	if len(actionKeys) == 0 {
		return nil, nil
	}
	return actions, actionKeys
}
