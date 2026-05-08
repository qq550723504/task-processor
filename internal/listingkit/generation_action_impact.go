package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

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
