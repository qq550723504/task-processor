package listingkit

type taskGenerationQueueReadPagePhase struct{}

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
