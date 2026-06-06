package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
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

func generationQueueBundles(result *ListingKitResult) []struct {
	platform string
	bundle   *common.PublishImageBundle
} {
	out := make([]struct {
		platform string
		bundle   *common.PublishImageBundle
	}, 0, 4)
	if result.Amazon != nil && result.Amazon.ImageBundle != nil {
		out = append(out, struct {
			platform string
			bundle   *common.PublishImageBundle
		}{platform: "amazon", bundle: result.Amazon.ImageBundle})
	}
	if result.Shein != nil && result.Shein.ImageBundle != nil {
		out = append(out, struct {
			platform string
			bundle   *common.PublishImageBundle
		}{platform: "shein", bundle: result.Shein.ImageBundle})
	}
	if result.Temu != nil && result.Temu.ImageBundle != nil {
		out = append(out, struct {
			platform string
			bundle   *common.PublishImageBundle
		}{platform: "temu", bundle: result.Temu.ImageBundle})
	}
	if result.Walmart != nil && result.Walmart.ImageBundle != nil {
		out = append(out, struct {
			platform string
			bundle   *common.PublishImageBundle
		}{platform: "walmart", bundle: result.Walmart.ImageBundle})
	}
	return out
}

func appendBundleQueueItems(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, renderPreviewIndex map[string]AssetRenderPreview, scenePresetIndex map[string]*GenerationScenePresetSummary, platform string, bundle *common.PublishImageBundle) {
	if bundle == nil {
		return
	}
	if bundle.Main != nil {
		appendBundleSlotQueueItem(items, index, renderPreviewIndex, scenePresetIndex, platform, *bundle.Main)
	}
	for _, slot := range bundle.Gallery {
		appendBundleSlotQueueItem(items, index, renderPreviewIndex, scenePresetIndex, platform, slot)
	}
	for _, slot := range bundle.Auxiliary {
		appendBundleSlotQueueItem(items, index, renderPreviewIndex, scenePresetIndex, platform, slot)
	}
	for _, slot := range bundle.MissingSlots {
		appendMissingSlotQueueItem(items, index, platform, slot)
	}
}

func appendBundleSlotQueueItem(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, renderPreviewIndex map[string]AssetRenderPreview, scenePresetIndex map[string]*GenerationScenePresetSummary, platform string, slot common.BundleSlot) {
	renderPreview := renderPreviewIndex[strings.TrimSpace(slot.AssetID)]
	item := GenerationWorkQueueItem{
		Platform:                 strings.TrimSpace(platform),
		GenerationTask:           "",
		Slot:                     strings.TrimSpace(slot.Key),
		Purpose:                  strings.TrimSpace(slot.Purpose),
		IdealKind:                strings.TrimSpace(slot.IdealKind),
		State:                    firstNonEmpty(slot.StateLabel, "ready"),
		SatisfiedBy:              strings.TrimSpace(slot.SatisfiedBy),
		IsFallback:               strings.EqualFold(slot.StateLabel, "fallback_in_use") || strings.EqualFold(slot.SatisfiedBy, "fallback_asset"),
		Retryable:                strings.EqualFold(slot.StateLabel, "fallback_in_use"),
		RecipeID:                 strings.TrimSpace(slot.RecipeID),
		TemplateLabel:            strings.TrimSpace(slot.TemplateLabel),
		AssetID:                  strings.TrimSpace(slot.AssetID),
		ExecutionState:           strings.TrimSpace(slot.ExecutionStatus),
		RetryHint:                strings.TrimSpace(slot.RetryHint),
		StateReason:              generationQueueSlotStateReason(slot),
		SelectedAssetID:          strings.TrimSpace(slot.AssetID),
		TargetAssetKind:          strings.TrimSpace(slot.IdealKind),
		ExecutionQuality:         generationQueueSlotExecutionQuality(slot),
		ExecutionQualityLabel:    generationExecutionQualityLabel(generationQueueSlotExecutionQuality(slot)),
		QualityGrade:             generationQualityGrade(generationQueueSlotExecutionQuality(slot)),
		QualityGradeLabel:        generationQualityGradeLabel(generationQualityGrade(generationQueueSlotExecutionQuality(slot))),
		RenderPreviewAvailable:   renderPreview.AssetID != "",
		RenderPreviewFormat:      renderPreview.PreviewFormat,
		RenderPreviewVisualMode:  renderPreview.VisualMode,
		RenderPreviewLayerTypes:  append([]string(nil), renderPreview.LayerTypes...),
		RenderPreviewRegions:     append([]string(nil), renderPreview.Regions...),
		RenderPreviewStyleTokens: append([]string(nil), renderPreview.StyleTokens...),
		ScenePreset:              cloneGenerationScenePresetSummary(scenePresetIndex[strings.TrimSpace(slot.AssetID)]),
	}
	item.PreviewCapabilities = buildRenderPreviewCapabilities(item)
	key := generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)
	index[key] = len(*items)
	*items = append(*items, item)
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

func appendMissingSlotQueueItem(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, platform string, slot common.MissingSlot) {
	item := GenerationWorkQueueItem{
		Platform:              strings.TrimSpace(platform),
		Slot:                  strings.TrimSpace(slot.Slot),
		Purpose:               strings.TrimSpace(slot.Purpose),
		State:                 firstNonEmpty(slot.StateLabel, "missing"),
		Retryable:             true,
		RecipeID:              strings.TrimSpace(slot.RecipeID),
		TemplateLabel:         strings.TrimSpace(slot.TemplateLabel),
		RenderProfile:         strings.TrimSpace(slot.RenderProfile),
		StateReason:           strings.TrimSpace(slot.Reason),
		ExecutionQuality:      "missing",
		ExecutionQualityLabel: generationExecutionQualityLabel("missing"),
		QualityGrade:          generationQualityGrade("missing"),
		QualityGradeLabel:     generationQualityGradeLabel(generationQualityGrade("missing")),
	}
	key := generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)
	if _, exists := index[key]; exists {
		return
	}
	index[key] = len(*items)
	*items = append(*items, item)
}

func generationQueueSlotStateReason(slot common.BundleSlot) string {
	switch strings.ToLower(strings.TrimSpace(slot.StateLabel)) {
	case "fallback_in_use":
		if value := strings.TrimSpace(slot.FallbackFrom); value != "" {
			return "using fallback asset while waiting for " + value
		}
		return "using fallback asset"
	case "ready":
		if value := strings.TrimSpace(slot.SatisfiedBy); value != "" {
			return "slot satisfied by " + value
		}
	}
	return ""
}

func generationQueueSlotExecutionQuality(slot common.BundleSlot) string {
	switch strings.ToLower(strings.TrimSpace(slot.StateLabel)) {
	case "fallback_in_use":
		return "fallback_asset"
	case "ready":
		return "exact_asset"
	default:
		return ""
	}
}
