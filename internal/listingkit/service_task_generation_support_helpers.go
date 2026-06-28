package listingkit

import (
	"context"
	"strings"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	listinggeneration "task-processor/internal/listingkit/generation"
)

func resolveLayerTemporalPlatform(req *ExecuteGenerationActionRequest) string {
	return resolveTemporalRequestPlatform(req)
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
			if !listinggeneration.TaskRetryable(item) {
				continue
			}
			out = append(out, listinggeneration.PrepareTaskRetry(item))
		}
		return out, nil
	}

	byID := make(map[string]assetgeneration.Task, len(existing))
	for _, item := range existing {
		byID[item.ID] = item
	}
	slotFilters := listinggeneration.NormalizeRetrySlots(req.Slots)
	filter := retrySelectionFilter(req)
	if len(req.TaskIDs) == 0 {
		out := make([]assetgeneration.Task, 0, len(existing))
		for _, item := range existing {
			if !listinggeneration.TaskRetryable(item) {
				continue
			}
			if !listinggeneration.MatchesTaskRetryFilter(item, retryQueueItem(queueIndex, item), slotFilters, filter) {
				continue
			}
			out = append(out, listinggeneration.PrepareTaskRetry(item))
		}
		return out, nil
	}

	out := make([]assetgeneration.Task, 0, len(req.TaskIDs))
	for _, id := range req.TaskIDs {
		item, ok := byID[id]
		if !ok {
			return nil, ErrGenerationTaskNotFound
		}
		if !listinggeneration.TaskRetryable(item) {
			return nil, ErrGenerationTaskNotRetryable
		}
		if !listinggeneration.MatchesTaskRetryFilter(item, retryQueueItem(queueIndex, item), slotFilters, filter) {
			return nil, ErrGenerationTaskNotRetryable
		}
		out = append(out, listinggeneration.PrepareTaskRetry(item))
	}
	return out, nil
}

func (s *service) buildRetryGenerationTaskSelection(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
	selected, err := selectGenerationTasksForRetry(existing, task.Result, req)
	if err != nil {
		return nil, err
	}
	planned, err := s.planMissingRetryGenerationTasks(ctx, task, inventory, existing, req)
	if err != nil {
		return nil, err
	}
	if len(planned) == 0 {
		return selected, nil
	}
	return listinggeneration.MergeRetrySelection(selected, planned), nil
}

func (s *service) planMissingRetryGenerationTasks(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
	assetGenerator := resolveWorkflowAssetGenerationService(s)
	assetRecipeResolver := resolveWorkflowAssetRecipeResolver(s)
	if s == nil || task == nil || task.Result == nil || assetGenerator == nil || assetRecipeResolver == nil {
		return nil, nil
	}
	recipesByPlatform := resolveRecipesForPlatforms(assetRecipeResolver, task.Request.Platforms, task.Result.CanonicalProduct)
	if len(recipesByPlatform) == 0 {
		return nil, nil
	}
	planned, err := assetGenerator.Plan(ctx, assetgeneration.Request{
		TaskID:    task.ID,
		Product:   effectiveCatalogProduct(task.Result),
		Inventory: inventory,
		Recipes:   flattenRecipes(recipesByPlatform),
	})
	if err != nil || planned == nil || len(planned.Tasks) == 0 {
		return nil, err
	}
	queueIndex := retryGenerationQueueIndex(existing, task.Result)
	slotFilters := listinggeneration.NormalizeRetrySlots(req.Slots)
	filter := retrySelectionFilter(req)
	existingKeys := make(map[generationQueueKey]struct{}, len(existing))
	for _, item := range existing {
		existingKeys[generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)] = struct{}{}
	}
	out := make([]assetgeneration.Task, 0, len(planned.Tasks))
	for _, item := range planned.Tasks {
		if !listinggeneration.TaskRetryable(item) {
			continue
		}
		key := generationQueueItemKey(item.Platform, item.RecipeID, item.Slot)
		if _, exists := existingKeys[key]; exists {
			continue
		}
		if _, ok := queueIndex[key]; !ok {
			continue
		}
		if !listinggeneration.MatchesTaskRetryFilter(item, retryQueueItem(queueIndex, item), slotFilters, filter) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func retryGenerationQueueIndex(existing []assetgeneration.Task, result *ListingKitResult) map[generationQueueKey]GenerationWorkQueueItem {
	queueResult := &ListingKitResult{AssetGenerationTasks: append([]assetgeneration.Task(nil), existing...)}
	if result != nil {
		queueResult.Amazon = result.Amazon
		queueResult.Shein = result.Shein
		queueResult.Temu = result.Temu
		queueResult.Walmart = result.Walmart
	}
	return indexGenerationWorkQueue(buildGenerationWorkQueue(queueResult))
}

func retrySelectionFilter(req *RetryGenerationTasksRequest) listinggeneration.RetrySelectionFilter {
	if req == nil {
		return listinggeneration.RetrySelectionFilter{}
	}
	return listinggeneration.RetrySelectionFilter{
		ExecutionQuality:      req.ExecutionQuality,
		ExecutionQualityLabel: req.ExecutionQualityLabel,
		QualityGrade:          req.QualityGrade,
		QualityGradeLabel:     req.QualityGradeLabel,
		FallbackOnly:          req.FallbackOnly,
		RendererOnly:          req.RendererOnly,
	}
}

func retryQueueItem(queueIndex map[generationQueueKey]GenerationWorkQueueItem, task assetgeneration.Task) *listinggeneration.RetryQueueItem {
	queueItem, ok := queueIndex[generationQueueItemKey(task.Platform, task.RecipeID, task.Slot)]
	if !ok {
		return nil
	}
	return &listinggeneration.RetryQueueItem{
		Slot:                  queueItem.Slot,
		State:                 queueItem.State,
		ExecutionMode:         queueItem.ExecutionMode,
		ExecutionQuality:      queueItem.ExecutionQuality,
		ExecutionQualityLabel: queueItem.ExecutionQualityLabel,
		QualityGrade:          queueItem.QualityGrade,
		QualityGradeLabel:     queueItem.QualityGradeLabel,
	}
}

func (s *service) listAssetGenerationTasks(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
	assetRepo := resolveWorkflowAssetRepository(s)
	if assetRepo == nil {
		return nil, nil
	}
	tasks, err := assetRepo.ListGenerationTasks(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return assetgeneration.CloneTasks(tasks), nil
}

func (s *service) listGenerationReviews(ctx context.Context, taskID string) ([]GenerationReviewRecord, error) {
	reviewRepo := resolveReviewRepository(s)
	if reviewRepo == nil {
		return nil, nil
	}
	records, err := reviewRepo.ListReviews(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	out := make([]GenerationReviewRecord, 0, len(records))
	for _, item := range records {
		out = append(out, GenerationReviewRecord{
			TaskID:          item.TaskID,
			Platform:        item.Platform,
			Slot:            item.Slot,
			Capability:      item.Capability,
			Decision:        GenerationReviewDecision(item.Decision),
			Status:          item.Status,
			Message:         item.Message,
			ReviewedAt:      item.ReviewedAt,
			ReviewedBy:      item.ReviewedBy,
			AssetID:         item.AssetID,
			AssetRevision:   item.AssetRevision,
			PreviewRevision: item.PreviewRevision,
			TaskRevision:    item.TaskRevision,
			SourceActionKey: item.SourceActionKey,
		})
	}
	return out, nil
}
