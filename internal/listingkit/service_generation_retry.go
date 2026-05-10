package listingkit

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

func (s *service) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, fmt.Errorf("listing kit result is not available")
	}
	inventory, err := s.assetRepo.GetInventory(ctx, asset.InventoryRef{TaskID: task.ID})
	if err != nil {
		return nil, err
	}
	existingTasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	selectedTasks, err := s.buildRetryGenerationTaskSelection(ctx, task, inventory, existingTasks, req)
	if err != nil {
		return nil, err
	}
	if len(selectedTasks) == 0 {
		page := buildGenerationTaskPage(task.ID, task.UpdatedAt, nil, nil, generationTaskListPage{
			Page:     defaultGenerationTaskPage,
			PageSize: defaultGenerationTaskPageSize,
			Total:    0,
		})
		page.MatchedQueue = &GenerationWorkQueue{Summary: &GenerationWorkQueueSummary{}}
		page.ExecutedQueue = &GenerationWorkQueue{Summary: &GenerationWorkQueueSummary{}}
		return page, nil
	}

	dispatchResult, err := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
		TaskID:    task.ID,
		Product:   effectiveCatalogProduct(task.Result),
		Inventory: inventory,
		Tasks:     selectedTasks,
	})
	if err != nil {
		return nil, err
	}

	updatedTasks := mergeGenerationTasks(existingTasks, dispatchResult.Tasks)
	reviews, err := s.listGenerationReviews(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	retriedTargets := generationTaskTargets(selectedTasks)
	inventory.Records = replaceGeneratedAssetsForTargets(inventory.Records, retriedTargets, dispatchResult.Assets)
	inventory.Summary = rebuildInventorySummary(inventory)

	if err := s.assetRepo.SaveInventory(ctx, inventory); err != nil {
		return nil, err
	}
	if err := s.assetRepo.SaveGenerationTasks(ctx, task.ID, updatedTasks); err != nil {
		return nil, err
	}

	rebuiltResult := *task.Result
	rebuiltResult.AssetBundle = rebuildBundleFromInventory(task.Result.AssetBundle, inventory)
	rebuiltResult.AssetInventorySummary = inventory.Summary
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, task.Result.CanonicalProduct)
	attachPlatformImageBundles(&rebuiltResult, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: updatedTasks}, s.assetBundleBuilder)
	decorateListingKitResultGeneration(&rebuiltResult, updatedTasks)
	syncAssetRenderPreviews(&rebuiltResult)
	if err := s.repo.SaveTaskResult(ctx, task.ID, &rebuiltResult); err != nil {
		return nil, err
	}
	decorateListingKitResultReview(&rebuiltResult, reviews)

	page := buildGenerationTaskPage(task.ID, task.UpdatedAt, updatedTasks, updatedTasks, generationTaskListPage{
		Page:     defaultGenerationTaskPage,
		PageSize: defaultGenerationTaskPageSize,
		Total:    len(updatedTasks),
	})
	page.MatchedQueue = buildMatchedGenerationQueue(rebuiltResult.AssetGenerationQueue, selectedTasks)
	page.ExecutedQueue = buildMatchedGenerationQueue(rebuiltResult.AssetGenerationQueue, dispatchResult.Tasks)
	return page, nil
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
