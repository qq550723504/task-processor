package listingkit

import (
	"context"
	"strings"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/catalog"
)

type generationTargetKey struct {
	RecipeID string
	Slot     string
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
