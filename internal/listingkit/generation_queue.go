package listingkit

import (
	"strings"
)

type generationQueueKey struct {
	Platform string
	RecipeID string
	Slot     string
}

func buildGenerationWorkQueue(result *ListingKitResult) *GenerationWorkQueue {
	if result == nil {
		return nil
	}
	items := make([]GenerationWorkQueueItem, 0, 16)
	index := make(map[generationQueueKey]int)
	renderPreviewIndex := indexAssetRenderPreviews(result)
	scenePresetIndex := indexGenerationScenePresets(result)
	for _, platformBundle := range generationQueueBundles(result) {
		appendBundleQueueItems(&items, index, renderPreviewIndex, scenePresetIndex, platformBundle.platform, platformBundle.bundle)
	}
	for _, task := range mergedGenerationQueueTasks(result) {
		mergeGenerationTaskIntoQueue(&items, index, task)
	}
	if len(items) == 0 {
		return nil
	}
	return &GenerationWorkQueue{
		Summary: buildGenerationWorkQueueSummary(items),
		Items:   items,
	}
}
func generationQueueItemKey(platform, recipeID, slot string) generationQueueKey {
	return generationQueueKey{
		Platform: strings.ToLower(strings.TrimSpace(platform)),
		RecipeID: strings.TrimSpace(recipeID),
		Slot:     strings.ToLower(strings.TrimSpace(slot)),
	}
}

func indexGenerationWorkQueue(queue *GenerationWorkQueue) map[generationQueueKey]GenerationWorkQueueItem {
	out := make(map[generationQueueKey]GenerationWorkQueueItem)
	if queue == nil {
		return out
	}
	for _, item := range queue.Items {
		out[generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)] = item
	}
	return out
}

func cloneGenerationScenePresetSummary(summary *GenerationScenePresetSummary) *GenerationScenePresetSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	return &cloned
}

func indexAssetRenderPreviews(result *ListingKitResult) map[string]AssetRenderPreview {
	out := make(map[string]AssetRenderPreview)
	if result == nil {
		return out
	}
	previews := result.AssetRenderPreviews
	if len(previews) == 0 {
		previews = attachTaskRevisionToAssetRenderPreviews(buildAssetRenderPreviews(result.AssetBundle), buildTaskRevision(result))
	}
	for _, preview := range previews {
		if assetID := strings.TrimSpace(preview.AssetID); assetID != "" {
			out[assetID] = preview
		}
	}
	return out
}

func indexGenerationScenePresets(result *ListingKitResult) map[string]*GenerationScenePresetSummary {
	out := make(map[string]*GenerationScenePresetSummary)
	if result == nil || result.AssetBundle == nil {
		return out
	}
	for _, item := range result.AssetBundle.Assets {
		assetID := strings.TrimSpace(item.ID)
		if assetID == "" {
			continue
		}
		summary := buildGenerationScenePresetSummaryFromMetadata(item.Metadata)
		if summary == nil {
			continue
		}
		out[assetID] = summary
	}
	return out
}
