package listingkit

import (
	"context"
	"strings"

	assetgeneration "task-processor/internal/asset/generation"
)

func (s *service) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error) {
	return s.taskGenerationOrDefault().RetryTaskGenerationTasks(ctx, taskID, req)
}

func selectGenerationTasksForRetry(existing []assetgeneration.Task, result *ListingKitResult, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
	if len(existing) == 0 {
		return nil, nil
	}
	queueResult := &ListingKitResult{AssetGenerationTasks: append([]assetgeneration.Task(nil), existing...)}
	if result != nil {
		queueResult.Amazon = result.Amazon
		queueResult.Shein = result.Shein
		queueResult.Temu = result.Temu
		queueResult.Walmart = result.Walmart
	}
	queueIndex := indexGenerationWorkQueue(buildGenerationWorkQueue(queueResult))
	if req == nil || (len(req.TaskIDs) == 0 &&
		len(req.Slots) == 0 &&
		strings.TrimSpace(req.ExecutionQuality) == "" &&
		strings.TrimSpace(req.ExecutionQualityLabel) == "" &&
		strings.TrimSpace(req.QualityGrade) == "" &&
		strings.TrimSpace(req.QualityGradeLabel) == "" &&
		!req.FallbackOnly &&
		!req.RendererOnly) {
		out := make([]assetgeneration.Task, 0, len(existing))
		for _, item := range existing {
			if !generationTaskRetryable(item) {
				continue
			}
			out = append(out, prepareGenerationTaskRetry(item))
		}
		return out, nil
	}

	byID := make(map[string]assetgeneration.Task, len(existing))
	for _, item := range existing {
		byID[item.ID] = item
	}
	slotFilters := normalizeRetrySlots(req.Slots)
	if len(req.TaskIDs) == 0 {
		out := make([]assetgeneration.Task, 0, len(existing))
		for _, item := range existing {
			if !generationTaskRetryable(item) {
				continue
			}
			if !generationTaskMatchesRetryFilter(item, queueIndex, slotFilters, req) {
				continue
			}
			out = append(out, prepareGenerationTaskRetry(item))
		}
		return out, nil
	}

	out := make([]assetgeneration.Task, 0, len(req.TaskIDs))
	for _, id := range req.TaskIDs {
		item, ok := byID[id]
		if !ok {
			return nil, ErrGenerationTaskNotFound
		}
		if !generationTaskRetryable(item) {
			return nil, ErrGenerationTaskNotRetryable
		}
		if !generationTaskMatchesRetryFilter(item, queueIndex, slotFilters, req) {
			return nil, ErrGenerationTaskNotRetryable
		}
		out = append(out, prepareGenerationTaskRetry(item))
	}
	return out, nil
}

func normalizeRetrySlots(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		out[value] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func generationTaskMatchesRetryFilter(task assetgeneration.Task, queueIndex map[generationQueueKey]GenerationWorkQueueItem, slotFilters map[string]struct{}, req *RetryGenerationTasksRequest) bool {
	if req == nil {
		return true
	}
	if len(slotFilters) > 0 {
		if _, ok := slotFilters[strings.ToLower(strings.TrimSpace(task.Slot))]; !ok {
			return false
		}
	}
	queueItem, ok := queueIndex[generationQueueItemKey(task.Platform, task.RecipeID, task.Slot)]
	if ok {
		return queueItemMatchesRetryRequest(queueItem, req)
	}
	if req.FallbackOnly && task.SatisfiedBy != "fallback_asset" && task.ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
		return false
	}
	if req.RendererOnly && task.ExecutionMode != assetgeneration.ExecutionModeRendererBacked && task.ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
		return false
	}
	return true
}

func prepareGenerationTaskRetry(task assetgeneration.Task) assetgeneration.Task {
	task.Status = "planned"
	task.ExecutionStatus = "planned"
	task.SatisfiedBy = ""
	task.FallbackFrom = ""
	task.ExecutionMode = assetgeneration.PlannedExecutionMode(task.AssetKind)
	return task
}
