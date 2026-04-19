package listingkit

import "strings"

func buildAssetGenerationActionImpact(queue *GenerationWorkQueue, query *GenerationQueueQuery) *AssetGenerationActionImpact {
	if queue == nil {
		return &AssetGenerationActionImpact{}
	}
	items := queue.Items
	if query != nil {
		items = filterGenerationQueueItems(items, query)
	}
	impact := &AssetGenerationActionImpact{
		MatchedItems: len(items),
	}
	platforms := make([]string, 0, len(items))
	grades := make([]string, 0, len(items))
	states := make([]string, 0, len(items))
	for _, item := range items {
		if item.Retryable {
			impact.RetryableItems++
		}
		platforms = append(platforms, item.Platform)
		grades = append(grades, item.QualityGrade)
		states = append(states, item.State)
	}
	impact.Platforms = uniqueStrings(platforms)
	impact.QualityGrades = uniqueNormalizedStrings(grades)
	impact.States = uniqueNormalizedStrings(states)
	return impact
}

func uniqueNormalizedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
