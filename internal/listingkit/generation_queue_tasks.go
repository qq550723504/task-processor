package listingkit

import (
	"strings"

	assetgeneration "task-processor/internal/asset/generation"
	listinggeneration "task-processor/internal/listingkit/generation"
)

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
		item.Retryable = listinggeneration.TaskRetryable(task)
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
		Retryable:             listinggeneration.TaskRetryable(task),
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
