package listingkit

import "context"

func (s *service) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	filtered := filterGenerationTasks(tasks, query)
	sorted := sortGenerationTasks(filtered, query)
	paged, meta := paginateGenerationTasks(sorted, query)
	return buildGenerationTaskPage(task.ID, task.UpdatedAt, filtered, paged, meta), nil
}

func (s *service) GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	reviews, err := s.listGenerationReviews(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	reviewedResult := withListingKitResultGenerationAndReview(task.Result, tasks, reviews)
	queue := reviewedResult.AssetGenerationQueue
	if queue == nil {
		page := buildGenerationQueuePage(task.ID, task.UpdatedAt, nil, nil, generationQueueListPage{
			Page:     resolveGenerationQueuePage(query),
			PageSize: resolveGenerationQueuePageSize(query),
			Total:    0,
		})
		page.DeltaToken = buildGenerationQueueDeltaToken(page, query)
		if isGenerationReviewReadNotModified(query, page.DeltaToken) {
			return applyGenerationConditionalStateToQueuePage(&GenerationQueuePage{
				TaskID:      task.ID,
				DeltaToken:  page.DeltaToken,
				NotModified: true,
				Page:        page.Page,
				PageSize:    page.PageSize,
				Total:       page.Total,
				UpdatedAt:   page.UpdatedAt,
			}), nil
		}
		return applyGenerationConditionalStateToQueuePage(page), nil
	}
	filtered := filterGenerationQueueItems(queue.Items, query)
	sorted := sortGenerationQueueItems(filtered, query)
	paged, meta := paginateGenerationQueueItems(sorted, query)
	page := buildGenerationQueuePage(task.ID, task.UpdatedAt, filtered, paged, meta)
	attachReviewSummaryToGenerationQueuePage(page, reviewedResult)
	page.DeltaToken = buildGenerationQueueDeltaToken(page, query)
	if isGenerationReviewReadNotModified(query, page.DeltaToken) {
		return applyGenerationConditionalStateToQueuePage(&GenerationQueuePage{
			TaskID:      task.ID,
			DeltaToken:  page.DeltaToken,
			NotModified: true,
			Page:        page.Page,
			PageSize:    page.PageSize,
			Total:       page.Total,
			UpdatedAt:   page.UpdatedAt,
		}), nil
	}
	return applyGenerationConditionalStateToQueuePage(page), nil
}
