package listingkit

import "strings"

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
