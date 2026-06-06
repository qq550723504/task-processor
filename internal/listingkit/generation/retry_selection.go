package generation

import (
	"strings"

	assetgeneration "task-processor/internal/asset/generation"
)

type RetrySelectionFilter struct {
	ExecutionQuality      string
	ExecutionQualityLabel string
	QualityGrade          string
	QualityGradeLabel     string
	FallbackOnly          bool
	RendererOnly          bool
}

type RetryQueueItem struct {
	Slot                  string
	State                 string
	ExecutionMode         string
	ExecutionQuality      string
	ExecutionQualityLabel string
	QualityGrade          string
	QualityGradeLabel     string
}

func NormalizeRetrySlots(values []string) map[string]struct{} {
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

func MatchesTaskRetryFilter(task assetgeneration.Task, queueItem *RetryQueueItem, slotFilters map[string]struct{}, filter RetrySelectionFilter) bool {
	if len(slotFilters) > 0 {
		if _, ok := slotFilters[strings.ToLower(strings.TrimSpace(task.Slot))]; !ok {
			return false
		}
	}
	if queueItem != nil {
		if filter.FallbackOnly && queueItem.State != "fallback_in_use" && queueItem.State != "stubbed" {
			return false
		}
		if !retryExecutionQualityMatches(queueItem, filter) {
			return false
		}
		if filter.RendererOnly &&
			queueItem.ExecutionMode != assetgeneration.ExecutionModeRendererBacked &&
			queueItem.ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
			return false
		}
		return true
	}
	if filter.FallbackOnly && task.SatisfiedBy != "fallback_asset" && task.ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
		return false
	}
	if filter.RendererOnly &&
		task.ExecutionMode != assetgeneration.ExecutionModeRendererBacked &&
		task.ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
		return false
	}
	return true
}

func PrepareTaskRetry(task assetgeneration.Task) assetgeneration.Task {
	task.Status = "planned"
	task.ExecutionStatus = "planned"
	task.SatisfiedBy = ""
	task.FallbackFrom = ""
	task.ExecutionMode = assetgeneration.PlannedExecutionMode(task.AssetKind)
	return task
}

func MergeRetrySelection(selected []assetgeneration.Task, planned []assetgeneration.Task) []assetgeneration.Task {
	if len(planned) == 0 {
		return selected
	}
	out := append([]assetgeneration.Task(nil), selected...)
	seen := make(map[retrySelectionKey]struct{}, len(selected)+len(planned))
	for _, item := range selected {
		seen[retrySelectionTaskKey(item)] = struct{}{}
	}
	for _, item := range planned {
		key := retrySelectionTaskKey(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

type retrySelectionKey struct {
	Platform string
	RecipeID string
	Slot     string
}

func retrySelectionTaskKey(task assetgeneration.Task) retrySelectionKey {
	return retrySelectionKey{
		Platform: strings.ToLower(strings.TrimSpace(task.Platform)),
		RecipeID: strings.TrimSpace(task.RecipeID),
		Slot:     strings.ToLower(strings.TrimSpace(task.Slot)),
	}
}

func retryExecutionQualityMatches(item *RetryQueueItem, filter RetrySelectionFilter) bool {
	if item == nil {
		return true
	}
	if value := strings.ToLower(strings.TrimSpace(filter.QualityGrade)); value != "" {
		if strings.ToLower(strings.TrimSpace(item.QualityGrade)) != value {
			return false
		}
	}
	if label := strings.ToLower(strings.TrimSpace(filter.QualityGradeLabel)); label != "" {
		if strings.ToLower(strings.TrimSpace(item.QualityGradeLabel)) != label {
			return false
		}
	}
	if value := strings.ToLower(strings.TrimSpace(filter.ExecutionQuality)); value != "" {
		if strings.ToLower(strings.TrimSpace(item.ExecutionQuality)) != value {
			return false
		}
	}
	if label := strings.ToLower(strings.TrimSpace(filter.ExecutionQualityLabel)); label != "" {
		if strings.ToLower(strings.TrimSpace(item.ExecutionQualityLabel)) != label {
			return false
		}
	}
	return true
}
