package listingkit

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
)

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

func (s *service) listAssetGenerationTasks(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
	if s.assetRepo == nil {
		return nil, nil
	}
	tasks, err := s.assetRepo.ListGenerationTasks(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return cloneGenerationTasks(tasks), nil
}

func (s *service) listGenerationReviews(ctx context.Context, taskID string) ([]GenerationReviewRecord, error) {
	if s.reviewRepo == nil {
		return nil, nil
	}
	records, err := s.reviewRepo.ListReviews(ctx, taskID)
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

func effectiveCatalogProduct(result *ListingKitResult) *catalog.Product {
	if result == nil {
		return nil
	}
	if result.CatalogProduct != nil {
		return result.CatalogProduct
	}
	return catalog.BuildProduct(result.CanonicalProduct)
}

func attachReviewSummaryToGenerationQueuePage(page *GenerationQueuePage, result *ListingKitResult) {
	if page == nil || page.Summary == nil || result == nil || result.ReviewSummary == nil {
		return
	}
	page.Summary.ApprovedSections = result.ReviewSummary.ApprovedSections
	page.Summary.DeferredSections = result.ReviewSummary.DeferredSections
	page.Summary.ReviewPendingSections = result.ReviewSummary.ReviewPendingSections
}

func decorateListingKitResultGeneration(result *ListingKitResult, tasks []assetgeneration.Task) {
	if result == nil {
		return
	}
	result.AssetGenerationTasks = cloneGenerationTasks(tasks)
	result.AssetGenerationSummary = buildAssetGenerationSummary(tasks)
	result.AssetGenerationQueue = buildGenerationWorkQueue(result)
	result.AssetGenerationOverview = buildAssetGenerationOverview(result.AssetGenerationQueue)
}

func withListingKitResultGeneration(result *ListingKitResult, tasks []assetgeneration.Task) *ListingKitResult {
	if result == nil {
		return &ListingKitResult{
			AssetGenerationTasks: cloneGenerationTasks(tasks),
		}
	}
	cloned := *result
	decorateListingKitResultGeneration(&cloned, tasks)
	return &cloned
}

func buildAssetGenerationSummary(tasks []assetgeneration.Task) *AssetGenerationSummary {
	summary := &AssetGenerationSummary{}
	if len(tasks) == 0 {
		return summary
	}
	platforms := make([]string, 0, len(tasks))
	for _, item := range tasks {
		summary.TotalTasks++
		switch strings.ToLower(strings.TrimSpace(item.ExecutionStatus)) {
		case "completed":
			summary.CompletedTasks++
		case "failed":
			summary.FailedTasks++
		default:
			summary.PlannedTasks++
		}
		switch item.ExecutionMode {
		case assetgeneration.ExecutionModeRendererBacked:
			summary.RendererBackedTasks++
		case assetgeneration.ExecutionModeDeferredStub:
			summary.FallbackTasks++
		}
		if generationTaskRetryable(item) {
			summary.RetryableTasks++
		}
		platforms = append(platforms, item.Platform)
	}
	summary.Platforms = uniqueStrings(platforms)
	return summary
}

func generationTaskRetryable(task assetgeneration.Task) bool {
	if !task.CanExecute {
		return false
	}
	if task.ExecutionStatus != "completed" {
		return true
	}
	switch task.ExecutionMode {
	case assetgeneration.ExecutionModeDeferredStub, assetgeneration.ExecutionModeRendererBacked:
		return true
	default:
		return false
	}
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

type generationTargetKey struct {
	RecipeID string
	Slot     string
}

func generationTaskTargets(tasks []assetgeneration.Task) map[generationTargetKey]struct{} {
	if len(tasks) == 0 {
		return nil
	}
	out := make(map[generationTargetKey]struct{}, len(tasks))
	for _, item := range tasks {
		recipeID := strings.TrimSpace(item.RecipeID)
		slot := strings.ToLower(strings.TrimSpace(item.Slot))
		if recipeID == "" {
			continue
		}
		out[generationTargetKey{RecipeID: recipeID, Slot: slot}] = struct{}{}
	}
	return out
}

func replaceGeneratedAssetsForTargets(existing []asset.AssetRecord, targets map[generationTargetKey]struct{}, updates []asset.AssetRecord) []asset.AssetRecord {
	if len(targets) == 0 {
		return append(append([]asset.AssetRecord(nil), existing...), updates...)
	}
	out := make([]asset.AssetRecord, 0, len(existing)+len(updates))
	for _, item := range existing {
		if item.Origin == asset.OriginGenerated {
			if _, ok := targets[assetTargetKey(item)]; ok {
				continue
			}
		}
		out = append(out, item)
	}
	out = append(out, updates...)
	return out
}

func assetTargetKey(item asset.AssetRecord) generationTargetKey {
	slot := ""
	if item.Metadata != nil {
		slot = firstNonEmpty(item.Metadata["bundle_slot"], item.Metadata["slot"])
	}
	return generationTargetKey{
		RecipeID: strings.TrimSpace(item.RecipeID),
		Slot:     strings.ToLower(strings.TrimSpace(slot)),
	}
}
