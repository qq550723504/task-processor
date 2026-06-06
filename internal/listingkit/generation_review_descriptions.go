package listingkit

type generationReviewSectionSpec struct {
	SectionKey  string
	Title       string
	Description string
	EmptyState  string
}

func generationReviewSectionKey(capability string) string {
	if capability == "" {
		return "general_review"
	}
	return capability
}

func generationReviewSectionSpecForCapability(capability string) generationReviewSectionSpec {
	cfg := generationReviewSectionConfigForCapability(capability)
	return generationReviewSectionSpec{
		SectionKey:  generationReviewSectionKey(capability),
		Title:       cfg.Title,
		Description: cfg.Description,
		EmptyState:  cfg.EmptyState,
	}
}

func generationPreviewCapabilityLabel(capability string) string {
	switch capability {
	case "detail_preview":
		return "Detail Preview"
	case "measurement_preview":
		return "Measurement Preview"
	case "badge_preview":
		return "Badge Preview"
	case "copy_preview":
		return "Copy Preview"
	case "subject_preview":
		return "Subject Preview"
	default:
		return capability
	}
}
