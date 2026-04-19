package listingkit

import (
	"strings"

	assetgeneration "task-processor/internal/asset/generation"
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
	for _, platformBundle := range generationQueueBundles(result) {
		appendBundleQueueItems(&items, index, renderPreviewIndex, platformBundle.platform, platformBundle.bundle)
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

func mergedGenerationQueueTasks(result *ListingKitResult) []assetgeneration.Task {
	if result == nil {
		return nil
	}
	byID := make(map[string]assetgeneration.Task)
	out := make([]assetgeneration.Task, 0, len(result.AssetGenerationTasks)+8)
	for _, task := range result.AssetGenerationTasks {
		if _, exists := byID[task.ID]; exists {
			continue
		}
		byID[task.ID] = task
		out = append(out, task)
	}
	for _, task := range collectPlatformGenerationTasks(result) {
		if _, exists := byID[task.ID]; exists {
			continue
		}
		byID[task.ID] = task
		out = append(out, task)
	}
	return out
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

func appendBundleQueueItems(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, renderPreviewIndex map[string]AssetRenderPreview, platform string, bundle *common.PublishImageBundle) {
	if bundle == nil {
		return
	}
	if bundle.Main != nil {
		appendBundleSlotQueueItem(items, index, renderPreviewIndex, platform, *bundle.Main)
	}
	for _, slot := range bundle.Gallery {
		appendBundleSlotQueueItem(items, index, renderPreviewIndex, platform, slot)
	}
	for _, slot := range bundle.Auxiliary {
		appendBundleSlotQueueItem(items, index, renderPreviewIndex, platform, slot)
	}
	for _, slot := range bundle.MissingSlots {
		appendMissingSlotQueueItem(items, index, platform, slot)
	}
}

func appendBundleSlotQueueItem(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, renderPreviewIndex map[string]AssetRenderPreview, platform string, slot common.BundleSlot) {
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

func mergeGenerationTaskIntoQueue(items *[]GenerationWorkQueueItem, index map[generationQueueKey]int, task assetgeneration.Task) {
	key := generationQueueItemKey(task.Platform, task.RecipeID, task.Slot)
	state := generationQueueStateFromTask(task)
	if idx, ok := index[key]; ok {
		item := (*items)[idx]
		item.TaskID = task.TaskID
		item.GenerationTask = task.ID
		item.Platform = firstNonEmpty(task.Platform, item.Platform)
		item.Slot = firstNonEmpty(task.Slot, item.Slot)
		item.Purpose = firstNonEmpty(task.Purpose, item.Purpose)
		item.IdealKind = firstNonEmpty(string(task.AssetKind), item.IdealKind)
		item.State = state
		item.SatisfiedBy = firstNonEmpty(task.SatisfiedBy, item.SatisfiedBy)
		item.IsFallback = item.IsFallback || state == "stubbed" || strings.EqualFold(task.SatisfiedBy, "fallback_asset")
		item.Retryable = generationTaskRetryable(task)
		item.RecipeID = firstNonEmpty(task.RecipeID, item.RecipeID)
		item.TemplateLabel = firstNonEmpty(task.TemplateLabel, item.TemplateLabel)
		item.RenderProfile = firstNonEmpty(task.RenderProfile, item.RenderProfile)
		item.ExecutionMode = task.ExecutionMode
		item.ExecutionState = task.ExecutionStatus
		item.StateReason = firstNonEmpty(generationQueueTaskStateReason(task), item.StateReason)
		item.TargetAssetKind = firstNonEmpty(string(task.AssetKind), item.TargetAssetKind)
		item.ExecutionQuality = firstNonEmpty(generationQueueTaskExecutionQuality(task), item.ExecutionQuality)
		item.ExecutionQualityLabel = firstNonEmpty(generationExecutionQualityLabel(generationQueueTaskExecutionQuality(task)), item.ExecutionQualityLabel)
		item.QualityGrade = firstNonEmpty(generationQualityGrade(generationQueueTaskExecutionQuality(task)), item.QualityGrade)
		item.QualityGradeLabel = firstNonEmpty(generationQualityGradeLabel(generationQualityGrade(generationQueueTaskExecutionQuality(task))), item.QualityGradeLabel)
		(*items)[idx] = item
		return
	}
	item := GenerationWorkQueueItem{
		TaskID:                task.TaskID,
		GenerationTask:        task.ID,
		Platform:              task.Platform,
		Slot:                  task.Slot,
		Purpose:               task.Purpose,
		IdealKind:             string(task.AssetKind),
		State:                 state,
		SatisfiedBy:           task.SatisfiedBy,
		IsFallback:            state == "stubbed" || strings.EqualFold(task.SatisfiedBy, "fallback_asset"),
		Retryable:             generationTaskRetryable(task),
		RecipeID:              task.RecipeID,
		TemplateLabel:         task.TemplateLabel,
		RenderProfile:         task.RenderProfile,
		ExecutionMode:         task.ExecutionMode,
		ExecutionState:        task.ExecutionStatus,
		StateReason:           generationQueueTaskStateReason(task),
		TargetAssetKind:       string(task.AssetKind),
		ExecutionQuality:      generationQueueTaskExecutionQuality(task),
		ExecutionQualityLabel: generationExecutionQualityLabel(generationQueueTaskExecutionQuality(task)),
		QualityGrade:          generationQualityGrade(generationQueueTaskExecutionQuality(task)),
		QualityGradeLabel:     generationQualityGradeLabel(generationQualityGrade(generationQueueTaskExecutionQuality(task))),
	}
	index[key] = len(*items)
	*items = append(*items, item)
}

func generationQueueStateFromTask(task assetgeneration.Task) string {
	switch strings.ToLower(strings.TrimSpace(task.ExecutionStatus)) {
	case "planned", "pending", "queued":
		return "queued"
	case "running", "processing", "in_progress":
		return "running"
	case "failed":
		return "failed"
	case "completed":
		if task.ExecutionMode == assetgeneration.ExecutionModeDeferredStub {
			return "stubbed"
		}
		return "completed"
	default:
		if task.ExecutionMode == assetgeneration.ExecutionModeDeferredStub {
			return "stubbed"
		}
		return "queued"
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

func generationQueueTaskStateReason(task assetgeneration.Task) string {
	switch strings.ToLower(strings.TrimSpace(task.ExecutionStatus)) {
	case "failed":
		return "generation task failed"
	case "running", "processing", "in_progress":
		return "generation task is running"
	case "planned", "pending", "queued":
		return "generation task is queued"
	case "completed":
		if task.ExecutionMode == assetgeneration.ExecutionModeDeferredStub {
			return "completed with stub fallback"
		}
		if task.ExecutionMode == assetgeneration.ExecutionModeRendererBacked {
			return "completed with renderer output"
		}
		return "generation task completed"
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

func generationQueueTaskExecutionQuality(task assetgeneration.Task) string {
	switch task.ExecutionMode {
	case assetgeneration.ExecutionModeRendererBacked:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "renderer_output"
		}
	case assetgeneration.ExecutionModeDeferredStub:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "stub_fallback"
		}
	case assetgeneration.ExecutionModePipelineBacked:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "pipeline_output"
		}
	case assetgeneration.ExecutionModeNativeAlias:
		if strings.EqualFold(strings.TrimSpace(task.ExecutionStatus), "completed") {
			return "alias_output"
		}
	}
	switch strings.ToLower(strings.TrimSpace(task.ExecutionStatus)) {
	case "failed":
		return "failed"
	case "running", "processing", "in_progress":
		return "running"
	case "planned", "pending", "queued":
		return "queued"
	}
	return ""
}
