package listingkit

import (
	"strings"

	listinggeneration "task-processor/internal/listingkit/generation"
)

func buildAssetGenerationActionTarget(queue *GenerationWorkQueue, actionKey string, filters *AssetGenerationRecommendedFilters) *AssetGenerationActionTarget {
	actionKey = strings.TrimSpace(actionKey)
	if actionKey == "" {
		return nil
	}
	target := &AssetGenerationActionTarget{
		ActionKey:       actionKey,
		InteractionMode: actionInteractionMode(actionKey),
	}
	cloned := cloneAssetGenerationFilters(actionFiltersForKey(actionKey, filters))
	if cloned == nil {
		return target
	}
	target.Filters = cloned
	target.QueueQuery = &GenerationQueueQuery{
		QualityGrade:                  cloned.QualityGrade,
		QualityGradeLabel:             cloned.QualityGradeLabel,
		ExecutionQuality:              cloned.ExecutionQuality,
		PreviewCapability:             cloned.PreviewCapability,
		RenderPreviewAvailable:        cloned.RenderPreviewAvailable,
		RenderPreviewAvailablePresent: cloned.RenderPreviewAvailable,
		Retryable:                     cloned.RetryableOnly,
		RetryablePresent:              cloned.RetryableOnly,
		SortBy:                        "quality_grade",
		SortOrder:                     "asc",
	}
	if len(cloned.Platforms) == 1 {
		target.QueueQuery.Platform = cloned.Platforms[0]
	}
	target.ExpectedImpact = buildAssetGenerationActionImpact(queue, target.QueueQuery)
	target.RetryRequest = &RetryGenerationTasksRequest{
		QualityGrade:      cloned.QualityGrade,
		QualityGradeLabel: cloned.QualityGradeLabel,
		ExecutionQuality:  cloned.ExecutionQuality,
	}
	switch target.InteractionMode {
	case "queue_only", "review_only":
		target.RetryRequest = nil
	}
	target.NavigationTarget = buildGenerationReviewActionNavigationTarget(target)
	return target
}

func buildSecondaryActionTargets(queue *GenerationWorkQueue, actionKeys []string, filters *AssetGenerationRecommendedFilters) []*AssetGenerationActionTarget {
	if len(actionKeys) == 0 {
		return nil
	}
	out := make([]*AssetGenerationActionTarget, 0, len(actionKeys))
	for _, actionKey := range uniqueStrings(actionKeys) {
		target := buildAssetGenerationActionTarget(queue, actionKey, filters)
		if target == nil {
			continue
		}
		out = append(out, target)
	}
	return out
}

func buildAssetGenerationActionImpact(queue *GenerationWorkQueue, query *GenerationQueueQuery) *AssetGenerationActionImpact {
	if queue == nil {
		return &AssetGenerationActionImpact{}
	}
	items := queue.Items
	if query != nil {
		items = filterGenerationQueueItems(items, query)
	}
	impactItems := make([]listinggeneration.ActionImpactItem, 0, len(items))
	for _, item := range items {
		impactItems = append(impactItems, listinggeneration.ActionImpactItem{
			Platform:     item.Platform,
			QualityGrade: item.QualityGrade,
			State:        item.State,
			Retryable:    item.Retryable,
		})
	}
	impact := listinggeneration.BuildActionImpact(impactItems)
	return &AssetGenerationActionImpact{
		MatchedItems:   impact.MatchedItems,
		RetryableItems: impact.RetryableItems,
		Platforms:      impact.Platforms,
		QualityGrades:  impact.QualityGrades,
		States:         impact.States,
	}
}

func actionInteractionMode(actionKey string) string {
	return listinggeneration.ActionInteractionMode(actionKey)
}

func buildPreviewCapabilitySecondaryActions(summary *GenerationWorkQueueSummary) ([]string, []string) {
	if summary == nil {
		return nil, nil
	}
	return listinggeneration.PreviewCapabilitySecondaryActions(summary.PreviewCapabilityCounts)
}
