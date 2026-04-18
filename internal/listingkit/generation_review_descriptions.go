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
