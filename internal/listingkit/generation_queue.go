package listingkit

import (
	"strings"

	listinggeneration "task-processor/internal/listingkit/generation"
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

func generationQualityGrade(value string) string {
	return listinggeneration.QualityGrade(value)
}

func generationQualityGradeLabel(value string) string {
	return listinggeneration.QualityGradeLabel(value)
}

func generationExecutionQualityLabel(value string) string {
	return listinggeneration.ExecutionQualityLabel(value)
}

func buildGenerationWorkQueueSummary(items []GenerationWorkQueueItem) *GenerationWorkQueueSummary {
	summary := &GenerationWorkQueueSummary{
		TotalItems:                      len(items),
		PlatformCounts:                  map[string]int{},
		PlatformPreviewableCounts:       map[string]int{},
		PreviewCapabilityCounts:         map[string]int{},
		PlatformPreviewCapabilityCounts: map[string]map[string]int{},
		StateCounts:                     map[string]int{},
		PlatformStateCounts:             map[string]map[string]int{},
		ExecutionQualityCounts:          map[string]int{},
		ExecutionQualityLabels:          map[string]string{},
		PlatformExecutionQualityCounts:  map[string]map[string]int{},
		QualityGradeCounts:              map[string]int{},
		QualityGradeLabels:              map[string]string{},
		PlatformQualityGradeCounts:      map[string]map[string]int{},
		GradeStateCounts:                map[string]map[string]int{},
		PlatformGradeStateCounts:        map[string]map[string]map[string]int{},
	}
	platforms := make([]string, 0, len(items))
	for _, item := range items {
		if platform := strings.TrimSpace(item.Platform); platform != "" {
			summary.PlatformCounts[platform]++
			if _, ok := summary.PlatformStateCounts[platform]; !ok {
				summary.PlatformStateCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformExecutionQualityCounts[platform]; !ok {
				summary.PlatformExecutionQualityCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformQualityGradeCounts[platform]; !ok {
				summary.PlatformQualityGradeCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformPreviewCapabilityCounts[platform]; !ok {
				summary.PlatformPreviewCapabilityCounts[platform] = map[string]int{}
			}
			if _, ok := summary.PlatformGradeStateCounts[platform]; !ok {
				summary.PlatformGradeStateCounts[platform] = map[string]map[string]int{}
			}
		}
		if item.RenderPreviewAvailable {
			summary.PreviewableItems++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformPreviewableCounts[platform]++
			}
			for _, capability := range item.PreviewCapabilities {
				summary.PreviewCapabilityCounts[capability]++
				if platform := strings.TrimSpace(item.Platform); platform != "" {
					summary.PlatformPreviewCapabilityCounts[platform][capability]++
				}
			}
		}
		if state := strings.TrimSpace(item.State); state != "" {
			summary.StateCounts[state]++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformStateCounts[platform][state]++
			}
		}
		if quality := strings.TrimSpace(item.ExecutionQuality); quality != "" {
			summary.ExecutionQualityCounts[quality]++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformExecutionQualityCounts[platform][quality]++
			}
			if label := firstNonEmpty(strings.TrimSpace(item.ExecutionQualityLabel), generationExecutionQualityLabel(quality)); label != "" {
				summary.ExecutionQualityLabels[quality] = label
			}
		}
		if grade := strings.TrimSpace(item.QualityGrade); grade != "" {
			summary.QualityGradeCounts[grade]++
			if platform := strings.TrimSpace(item.Platform); platform != "" {
				summary.PlatformQualityGradeCounts[platform][grade]++
			}
			if label := firstNonEmpty(strings.TrimSpace(item.QualityGradeLabel), generationQualityGradeLabel(grade)); label != "" {
				summary.QualityGradeLabels[grade] = label
			}
			if _, ok := summary.GradeStateCounts[grade]; !ok {
				summary.GradeStateCounts[grade] = map[string]int{}
			}
			if state := strings.TrimSpace(item.State); state != "" {
				summary.GradeStateCounts[grade][state]++
				if platform := strings.TrimSpace(item.Platform); platform != "" {
					if _, ok := summary.PlatformGradeStateCounts[platform][grade]; !ok {
						summary.PlatformGradeStateCounts[platform][grade] = map[string]int{}
					}
					summary.PlatformGradeStateCounts[platform][grade][state]++
				}
			}
		}
		switch item.State {
		case "ready":
			summary.ReadyItems++
		case "fallback_in_use":
			summary.FallbackItems++
		case "missing":
			summary.MissingItems++
		case "queued":
			summary.QueuedItems++
		case "running":
			summary.RunningItems++
		case "completed":
			summary.CompletedItems++
		case "failed":
			summary.FailedItems++
		case "stubbed":
			summary.StubbedItems++
		}
		if item.Retryable {
			summary.RetryableItems++
		}
		switch strings.ToLower(strings.TrimSpace(item.ReviewStatus)) {
		case "approved":
			summary.ApprovedSections++
		case "deferred":
			summary.DeferredSections++
		case "pending":
			summary.ReviewPendingSections++
		default:
			if item.RenderPreviewAvailable {
				summary.ReviewPendingSections++
			}
		}
		platforms = append(platforms, item.Platform)
	}
	summary.Platforms = uniqueStrings(platforms)
	summary.DominantQualityGrade = dominantGenerationQualityGrade(summary)
	summary.DominantQualityGradeLabel = generationQualityGradeLabel(summary.DominantQualityGrade)
	return summary
}
