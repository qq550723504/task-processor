package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

type taskGenerationQueueReadResponsePhase struct{}

func buildTaskGenerationQueueReadResponsePhase() *taskGenerationQueueReadResponsePhase {
	return &taskGenerationQueueReadResponsePhase{}
}

func (p *taskGenerationQueueReadResponsePhase) run(taskID string, page *GenerationQueuePage, query *GenerationQueueQuery) *GenerationQueuePage {
	if page == nil {
		page = &GenerationQueuePage{TaskID: taskID}
	}
	page.DeltaToken = buildGenerationQueueDeltaToken(page, query)
	if query != nil && listinggeneration.IsReadNotModified(query.DeltaToken, query.IfMatch, page.DeltaToken) {
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
