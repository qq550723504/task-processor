package listingkit

import "strings"

func buildPlatformGenerationWorkQueue(queue *GenerationWorkQueue, platform string) *GenerationWorkQueue {
	if queue == nil || strings.TrimSpace(platform) == "" {
		return nil
	}
	items := make([]GenerationWorkQueueItem, 0, len(queue.Items))
	for _, item := range queue.Items {
		if item.Platform == platform {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &GenerationWorkQueue{
		Summary: buildGenerationWorkQueueSummary(items),
		Items:   items,
	}
}

func clonePlatformAssetRenderPreviewSummary(summary *PlatformAssetRenderPreviewSummary) *PlatformAssetRenderPreviewSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.CapabilityCounts = cloneStringIntMap(summary.CapabilityCounts)
	cloned.VisualModes = append([]string(nil), summary.VisualModes...)
	return &cloned
}
