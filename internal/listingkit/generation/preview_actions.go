package generation

type PreviewCapabilityActionSpec struct {
	Capability string
	ActionKey  string
	Label      string
}

func PreviewCapabilityActionSpecs() []PreviewCapabilityActionSpec {
	return []PreviewCapabilityActionSpec{
		{Capability: "detail_preview", ActionKey: ActionReviewDetailPreviews, Label: "Review Detail Previews"},
		{Capability: "measurement_preview", ActionKey: ActionReviewMeasurementPreviews, Label: "Review Measurement Previews"},
		{Capability: "badge_preview", ActionKey: ActionReviewBadgePreviews, Label: "Review Badge Previews"},
		{Capability: "copy_preview", ActionKey: ActionReviewCopyPreviews, Label: "Review Copy Previews"},
		{Capability: "subject_preview", ActionKey: ActionReviewSubjectPreviews, Label: "Review Subject Previews"},
	}
}

func PreviewCapabilityActionSpecForKey(actionKey string) *PreviewCapabilityActionSpec {
	for _, spec := range PreviewCapabilityActionSpecs() {
		if spec.ActionKey == actionKey {
			copySpec := spec
			return &copySpec
		}
	}
	return nil
}

func ReviewActionKeyForCapability(capability string) string {
	if spec := PreviewCapabilityActionSpecForKey(CapabilityActionKey(capability)); spec != nil {
		return spec.ActionKey
	}
	return ActionReviewReadyAssets
}

func CapabilityActionKey(capability string) string {
	switch capability {
	case "detail_preview":
		return ActionReviewDetailPreviews
	case "measurement_preview":
		return ActionReviewMeasurementPreviews
	case "badge_preview":
		return ActionReviewBadgePreviews
	case "copy_preview":
		return ActionReviewCopyPreviews
	case "subject_preview":
		return ActionReviewSubjectPreviews
	default:
		return ""
	}
}

func ReviewActionLabelForCapability(capability string) string {
	if spec := PreviewCapabilityActionSpecForKey(CapabilityActionKey(capability)); spec != nil {
		return spec.Label
	}
	return "Review Previews"
}

func PreviewCapabilitySecondaryActions(counts map[string]int) ([]string, []string) {
	if len(counts) == 0 {
		return nil, nil
	}
	actions := make([]string, 0, len(counts))
	actionKeys := make([]string, 0, len(counts))
	for _, spec := range PreviewCapabilityActionSpecs() {
		if counts[spec.Capability] <= 0 {
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
