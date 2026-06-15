package listingkit

import (
	"strings"

	common "task-processor/internal/publishing/common"
)

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
	quality := generationQueueSlotExecutionQuality(slot)
	grade := generationQualityGrade(quality)
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
		ExecutionQuality:         quality,
		ExecutionQualityLabel:    generationExecutionQualityLabel(quality),
		QualityGrade:             grade,
		QualityGradeLabel:        generationQualityGradeLabel(grade),
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

func appendMissingSlotQueueItem(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, platform string, slot common.MissingSlot) {
	grade := generationQualityGrade("missing")
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
		QualityGrade:          grade,
		QualityGradeLabel:     generationQualityGradeLabel(grade),
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
