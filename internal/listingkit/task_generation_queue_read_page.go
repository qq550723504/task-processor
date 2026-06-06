package listingkit

import "time"

type taskGenerationQueueReadPagePhase struct{}

type generationQueueListPage struct {
	Page     int
	PageSize int
	Total    int
}

func buildTaskGenerationQueueReadPagePhase() *taskGenerationQueueReadPagePhase {
	return &taskGenerationQueueReadPagePhase{}
}

func (p *taskGenerationQueueReadPagePhase) run(snapshot *taskGenerationQueueReadSnapshot, query *GenerationQueueQuery) *GenerationQueuePage {
	if snapshot == nil {
		snapshot = &taskGenerationQueueReadSnapshot{task: &Task{}}
	}
	if snapshot.task == nil {
		snapshot.task = &Task{}
	}
	if snapshot.queue == nil {
		return buildGenerationQueuePage(snapshot.task.ID, snapshot.task.UpdatedAt, nil, nil, generationQueueListPage{
			Page:     resolveGenerationQueuePage(query),
			PageSize: resolveGenerationQueuePageSize(query),
			Total:    0,
		})
	}

	filtered := filterGenerationQueueItems(snapshot.queue.Items, query)
	sorted := sortGenerationQueueItems(filtered, query)
	paged, meta := paginateGenerationQueueItems(sorted, query)
	page := buildGenerationQueuePage(snapshot.task.ID, snapshot.task.UpdatedAt, filtered, paged, meta)
	attachReviewSummaryToGenerationQueuePage(page, snapshot.result)
	return page
}

func attachReviewSummaryToGenerationQueuePage(page *GenerationQueuePage, result *ListingKitResult) {
	if page == nil || page.Summary == nil || result == nil || result.ReviewSummary == nil {
		return
	}
	page.Summary.ApprovedSections = result.ReviewSummary.ApprovedSections
	page.Summary.DeferredSections = result.ReviewSummary.DeferredSections
	page.Summary.ReviewPendingSections = result.ReviewSummary.ReviewPendingSections
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
