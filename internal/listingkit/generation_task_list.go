package listingkit

import (
	"sort"
	"strings"
	"time"

	assetgeneration "task-processor/internal/asset/generation"
)

const (
	defaultGenerationTaskPage     = 1
	defaultGenerationTaskPageSize = 20
	maxGenerationTaskPageSize     = 100
)

type generationTaskListPage struct {
	Page     int
	PageSize int
	Total    int
}

func filterGenerationTasks(tasks []assetgeneration.Task, query *GenerationTaskQuery) []assetgeneration.Task {
	if len(tasks) == 0 || query == nil {
		return append([]assetgeneration.Task(nil), tasks...)
	}
	platform := strings.ToLower(strings.TrimSpace(query.Platform))
	slot := strings.ToLower(strings.TrimSpace(query.Slot))
	executionMode := strings.ToLower(strings.TrimSpace(query.ExecutionMode))
	executionStatus := strings.ToLower(strings.TrimSpace(query.ExecutionStatus))
	satisfiedBy := strings.ToLower(strings.TrimSpace(query.SatisfiedBy))
	if platform == "" && slot == "" && executionMode == "" && executionStatus == "" && satisfiedBy == "" {
		return append([]assetgeneration.Task(nil), tasks...)
	}
	out := make([]assetgeneration.Task, 0, len(tasks))
	for _, item := range tasks {
		if platform != "" && strings.ToLower(strings.TrimSpace(item.Platform)) != platform {
			continue
		}
		if slot != "" && strings.ToLower(strings.TrimSpace(item.Slot)) != slot {
			continue
		}
		if executionMode != "" && strings.ToLower(strings.TrimSpace(item.ExecutionMode)) != executionMode {
			continue
		}
		if executionStatus != "" && strings.ToLower(strings.TrimSpace(item.ExecutionStatus)) != executionStatus {
			continue
		}
		if satisfiedBy != "" && strings.ToLower(strings.TrimSpace(item.SatisfiedBy)) != satisfiedBy {
			continue
		}
		out = append(out, item)
	}
	return out
}

func sortGenerationTasks(tasks []assetgeneration.Task, query *GenerationTaskQuery) []assetgeneration.Task {
	out := append([]assetgeneration.Task(nil), tasks...)
	if len(out) < 2 {
		return out
	}
	sortBy := "updated_at"
	sortOrder := "desc"
	if query != nil {
		if value := strings.ToLower(strings.TrimSpace(query.SortBy)); value != "" {
			sortBy = value
		}
		if value := strings.ToLower(strings.TrimSpace(query.SortOrder)); value == "asc" || value == "desc" {
			sortOrder = value
		}
	}
	if sortBy == "updated_at" {
		if sortOrder == "asc" {
			slicesReverseTasks(out)
		}
		return out
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := out[i], out[j]
		var less bool
		switch sortBy {
		case "platform":
			less = compareTaskSortValue(left.Platform, right.Platform, left.ID, right.ID)
		case "slot":
			less = compareTaskSortValue(left.Slot, right.Slot, left.ID, right.ID)
		case "execution_status":
			less = compareTaskSortValue(left.ExecutionStatus, right.ExecutionStatus, left.ID, right.ID)
		default:
			less = compareTaskSortValue(left.ID, right.ID, "", "")
		}
		if sortOrder == "desc" {
			return !less
		}
		return less
	})
	return out
}

func paginateGenerationTasks(tasks []assetgeneration.Task, query *GenerationTaskQuery) ([]assetgeneration.Task, generationTaskListPage) {
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
	total := len(tasks)
	if total == 0 {
		return nil, generationTaskListPage{Page: page, PageSize: pageSize, Total: 0}
	}
	start := (page - 1) * pageSize
	if start >= total {
		return nil, generationTaskListPage{Page: page, PageSize: pageSize, Total: total}
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return append([]assetgeneration.Task(nil), tasks[start:end]...), generationTaskListPage{Page: page, PageSize: pageSize, Total: total}
}

func buildGenerationTaskPage(taskID string, updatedAt time.Time, filtered []assetgeneration.Task, paged []assetgeneration.Task, page generationTaskListPage) *GenerationTaskPage {
	return &GenerationTaskPage{
		TaskID:    taskID,
		Summary:   buildAssetGenerationSummary(filtered),
		Page:      page.Page,
		PageSize:  page.PageSize,
		Total:     page.Total,
		Tasks:     append([]assetgeneration.Task(nil), paged...),
		UpdatedAt: updatedAt,
	}
}

func compareTaskSortValue(left, right, leftID, rightID string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if left == right {
		return strings.ToLower(strings.TrimSpace(leftID)) < strings.ToLower(strings.TrimSpace(rightID))
	}
	return left < right
}

func slicesReverseTasks(tasks []assetgeneration.Task) {
	for i, j := 0, len(tasks)-1; i < j; i, j = i+1, j-1 {
		tasks[i], tasks[j] = tasks[j], tasks[i]
	}
}
