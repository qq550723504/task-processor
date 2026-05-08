package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildPreviewCapabilitySecondaryActions(summary *GenerationWorkQueueSummary) ([]string, []string) {
	if summary == nil {
		return nil, nil
	}
	return listinggeneration.PreviewCapabilitySecondaryActions(summary.PreviewCapabilityCounts)
}
