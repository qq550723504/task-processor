package listingkit

import (
	"strings"

	assetgeneration "task-processor/internal/asset/generation"
)

func buildMatchedGenerationQueue(queue *GenerationWorkQueue, tasks []assetgeneration.Task) *GenerationWorkQueue {
	if queue == nil || len(tasks) == 0 {
		return &GenerationWorkQueue{Summary: buildGenerationWorkQueueSummary(nil)}
	}
	keys := make(map[generationQueueKey]struct{}, len(tasks))
	for _, task := range tasks {
		keys[generationQueueItemKey(task.Platform, task.RecipeID, task.Slot)] = struct{}{}
	}
	items := make([]GenerationWorkQueueItem, 0, len(tasks))
	for _, item := range queue.Items {
		if _, ok := keys[generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)]; !ok {
			continue
		}
		items = append(items, item)
	}
	return &GenerationWorkQueue{
		Summary: buildGenerationWorkQueueSummary(items),
		Items:   items,
	}
}

func resolveGenerationQueuePage(query *GenerationQueueQuery) int {
	if query != nil && query.Page > 0 {
		return query.Page
	}
	return defaultGenerationTaskPage
}

func resolveGenerationQueuePageSize(query *GenerationQueueQuery) int {
	if query != nil && query.PageSize > 0 {
		if query.PageSize > maxGenerationTaskPageSize {
			return maxGenerationTaskPageSize
		}
		return query.PageSize
	}
	return defaultGenerationTaskPageSize
}

func queueItemMatchesRetryRequest(item GenerationWorkQueueItem, req *RetryGenerationTasksRequest) bool {
	if req == nil {
		return true
	}
	if len(req.Slots) > 0 {
		matched := false
		for _, slot := range req.Slots {
			if strings.EqualFold(strings.TrimSpace(slot), strings.TrimSpace(item.Slot)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if req.FallbackOnly && item.State != "fallback_in_use" && item.State != "stubbed" {
		return false
	}
	if !retryExecutionQualityMatches(item, req) {
		return false
	}
	if req.RendererOnly && item.ExecutionMode != assetgeneration.ExecutionModeRendererBacked && item.ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
		return false
	}
	return true
}

func retryExecutionQualityMatches(item GenerationWorkQueueItem, req *RetryGenerationTasksRequest) bool {
	if req == nil {
		return true
	}
	if value := strings.ToLower(strings.TrimSpace(req.QualityGrade)); value != "" {
		if strings.ToLower(strings.TrimSpace(item.QualityGrade)) != value {
			return false
		}
	}
	if label := strings.ToLower(strings.TrimSpace(req.QualityGradeLabel)); label != "" {
		itemLabel := strings.ToLower(strings.TrimSpace(item.QualityGradeLabel))
		if itemLabel == "" {
			itemLabel = strings.ToLower(strings.TrimSpace(generationQualityGradeLabel(item.QualityGrade)))
		}
		if itemLabel != label {
			return false
		}
	}
	if value := strings.ToLower(strings.TrimSpace(req.ExecutionQuality)); value != "" {
		if strings.ToLower(strings.TrimSpace(item.ExecutionQuality)) != value {
			return false
		}
	}
	if label := strings.ToLower(strings.TrimSpace(req.ExecutionQualityLabel)); label != "" {
		itemLabel := strings.ToLower(strings.TrimSpace(item.ExecutionQualityLabel))
		if itemLabel == "" {
			itemLabel = strings.ToLower(strings.TrimSpace(generationExecutionQualityLabel(item.ExecutionQuality)))
		}
		if itemLabel != label {
			return false
		}
	}
	return true
}
