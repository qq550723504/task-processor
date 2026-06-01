package listingkit

type taskGenerationQueueReadResponsePhase struct{}

func buildTaskGenerationQueueReadResponsePhase() *taskGenerationQueueReadResponsePhase {
	return &taskGenerationQueueReadResponsePhase{}
}

func (p *taskGenerationQueueReadResponsePhase) run(taskID string, page *GenerationQueuePage, query *GenerationQueueQuery) *GenerationQueuePage {
	if page == nil {
		page = &GenerationQueuePage{TaskID: taskID}
	}
	page.DeltaToken = buildGenerationQueueDeltaToken(page, query)
	if isGenerationReviewReadNotModified(query, page.DeltaToken) {
		return applyGenerationConditionalStateToQueuePage(&GenerationQueuePage{
			TaskID:      page.TaskID,
			DeltaToken:  page.DeltaToken,
			NotModified: true,
			Page:        page.Page,
			PageSize:    page.PageSize,
			Total:       page.Total,
			UpdatedAt:   page.UpdatedAt,
		})
	}
	return applyGenerationConditionalStateToQueuePage(page)
}
