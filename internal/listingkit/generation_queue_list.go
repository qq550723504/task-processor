package listingkit

import (
	"sort"
	"strings"
	"time"
)

type generationQueueListPage struct {
	Page     int
	PageSize int
	Total    int
}

func filterGenerationQueueItems(items []GenerationWorkQueueItem, query *GenerationQueueQuery) []GenerationWorkQueueItem {
	if len(items) == 0 || query == nil {
		return append([]GenerationWorkQueueItem(nil), items...)
	}
	platform := strings.ToLower(strings.TrimSpace(query.Platform))
	slot := strings.ToLower(strings.TrimSpace(query.Slot))
	state := strings.ToLower(strings.TrimSpace(query.State))
	executionMode := strings.ToLower(strings.TrimSpace(query.ExecutionMode))
	executionQuality := strings.ToLower(strings.TrimSpace(query.ExecutionQuality))
	qualityGrade := strings.ToLower(strings.TrimSpace(query.QualityGrade))
	qualityGradeLabel := strings.ToLower(strings.TrimSpace(query.QualityGradeLabel))
	previewCapability := strings.ToLower(strings.TrimSpace(query.PreviewCapability))
	if platform == "" && slot == "" && state == "" && executionMode == "" && executionQuality == "" && qualityGrade == "" && qualityGradeLabel == "" && previewCapability == "" && !query.RetryablePresent && !query.RenderPreviewAvailablePresent {
		return append([]GenerationWorkQueueItem(nil), items...)
	}
	out := make([]GenerationWorkQueueItem, 0, len(items))
	for _, item := range items {
		if platform != "" && strings.ToLower(strings.TrimSpace(item.Platform)) != platform {
			continue
		}
		if slot != "" && strings.ToLower(strings.TrimSpace(item.Slot)) != slot {
			continue
		}
		if state != "" && strings.ToLower(strings.TrimSpace(item.State)) != state {
			continue
		}
		if executionMode != "" && strings.ToLower(strings.TrimSpace(item.ExecutionMode)) != executionMode {
			continue
		}
		if executionQuality != "" && strings.ToLower(strings.TrimSpace(item.ExecutionQuality)) != executionQuality {
			continue
		}
		if qualityGrade != "" && strings.ToLower(strings.TrimSpace(item.QualityGrade)) != qualityGrade {
			continue
		}
		if qualityGradeLabel != "" && strings.ToLower(strings.TrimSpace(item.QualityGradeLabel)) != qualityGradeLabel {
			continue
		}
		if previewCapability != "" && !queueItemHasPreviewCapability(item, previewCapability) {
			continue
		}
		if query.RenderPreviewAvailablePresent && item.RenderPreviewAvailable != query.RenderPreviewAvailable {
			continue
		}
		if query.RetryablePresent && item.Retryable != query.Retryable {
			continue
		}
		out = append(out, item)
	}
	return out
}

func sortGenerationQueueItems(items []GenerationWorkQueueItem, query *GenerationQueueQuery) []GenerationWorkQueueItem {
	out := append([]GenerationWorkQueueItem(nil), items...)
	if len(out) < 2 {
		return out
	}
	sortBy := "platform"
	sortOrder := "asc"
	if query != nil {
		if value := strings.ToLower(strings.TrimSpace(query.SortBy)); value != "" {
			sortBy = value
		}
		if value := strings.ToLower(strings.TrimSpace(query.SortOrder)); value == "asc" || value == "desc" {
			sortOrder = value
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := out[i], out[j]
		var less bool
		switch sortBy {
		case "slot":
			less = compareQueueSortValue(left.Slot, right.Slot, left.Platform+left.RecipeID, right.Platform+right.RecipeID)
		case "state":
			less = compareQueueSortValue(left.State, right.State, left.Platform+left.Slot, right.Platform+right.Slot)
		case "quality_grade":
			less = compareQueueSortValue(left.QualityGrade, right.QualityGrade, left.Platform+left.Slot, right.Platform+right.Slot)
		case "execution_quality":
			less = compareQueueSortValue(left.ExecutionQuality, right.ExecutionQuality, left.Platform+left.Slot, right.Platform+right.Slot)
		case "retryable":
			less = compareQueueBoolValue(left.Retryable, right.Retryable, left.Platform+left.Slot, right.Platform+right.Slot)
		case "render_preview_available":
			less = compareQueueBoolValue(left.RenderPreviewAvailable, right.RenderPreviewAvailable, left.Platform+left.Slot, right.Platform+right.Slot)
		case "template_label":
			less = compareQueueSortValue(left.TemplateLabel, right.TemplateLabel, left.Platform+left.Slot, right.Platform+right.Slot)
		default:
			less = compareQueueSortValue(left.Platform, right.Platform, left.Slot+left.RecipeID, right.Slot+right.RecipeID)
		}
		if sortOrder == "desc" {
			return !less
		}
		return less
	})
	return out
}

func queueItemHasPreviewCapability(item GenerationWorkQueueItem, capability string) bool {
	for _, value := range item.PreviewCapabilities {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(capability)) {
			return true
		}
	}
	return false
}

func paginateGenerationQueueItems(items []GenerationWorkQueueItem, query *GenerationQueueQuery) ([]GenerationWorkQueueItem, generationQueueListPage) {
	page := defaultGenerationTaskPage
	pageSize := defaultGenerationTaskPageSize
	if query != nil {
		if query.Page > 0 {
			page = query.Page
		}
		if query.PageSize > 0 {
			pageSize = query.PageSize
		}
	}
	if pageSize > maxGenerationTaskPageSize {
		pageSize = maxGenerationTaskPageSize
	}
	total := len(items)
	if total == 0 {
		return nil, generationQueueListPage{Page: page, PageSize: pageSize, Total: 0}
	}
	start := (page - 1) * pageSize
	if start >= total {
		return nil, generationQueueListPage{Page: page, PageSize: pageSize, Total: total}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return append([]GenerationWorkQueueItem(nil), items[start:end]...), generationQueueListPage{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}
}

func buildGenerationQueuePage(taskID string, updatedAt time.Time, filtered []GenerationWorkQueueItem, paged []GenerationWorkQueueItem, page generationQueueListPage) *GenerationQueuePage {
	return &GenerationQueuePage{
		TaskID:    taskID,
		Summary:   buildGenerationWorkQueueSummary(filtered),
		Page:      page.Page,
		PageSize:  page.PageSize,
		Total:     page.Total,
		Items:     append([]GenerationWorkQueueItem(nil), paged...),
		UpdatedAt: updatedAt,
	}
}

func compareQueueSortValue(left, right, leftID, rightID string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if left == right {
		return strings.ToLower(strings.TrimSpace(leftID)) < strings.ToLower(strings.TrimSpace(rightID))
	}
	return left < right
}

func compareQueueBoolValue(left, right bool, leftID, rightID string) bool {
	if left == right {
		return strings.ToLower(strings.TrimSpace(leftID)) < strings.ToLower(strings.TrimSpace(rightID))
	}
	return !left && right
}
