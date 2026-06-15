package listingkit

import "testing"

func TestGenerationOverviewQualitySupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "generation_overview.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func buildAssetGenerationOverview(queue *GenerationWorkQueue) *AssetGenerationOverview {",
		"func reviewActionKey(summary *GenerationWorkQueueSummary) string {",
		"func actionFiltersForKey(actionKey string, base *AssetGenerationRecommendedFilters) *AssetGenerationRecommendedFilters {",
		"func applyAssetGenerationActionFiltersMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {",
		"func applyAssetGenerationRegularActionKeyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func cloneStringIntMap(input map[string]int) map[string]int {",
		"func previewReadyPlatformsForQueue(summary *GenerationWorkQueueSummary) []string {",
		"func previewReadyCapabilitiesForQueue(summary *GenerationWorkQueueSummary) []string {",
		"func reviewActionReason(summary *GenerationWorkQueueSummary) string {",
		"func dominantGenerationQualityGrade(summary *GenerationWorkQueueSummary) string {",
		"func blockingPlatformsForQueue(queue *GenerationWorkQueue) []string {",
		"func blockingQualityGradesForQueue(summary *GenerationWorkQueueSummary) []string {",
	})

	supportSource := readTaskGenerationSourceFile(t, "generation_overview_quality_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func cloneStringIntMap(input map[string]int) map[string]int {",
		"func previewReadyPlatformsForQueue(summary *GenerationWorkQueueSummary) []string {",
		"func previewReadyCapabilitiesForQueue(summary *GenerationWorkQueueSummary) []string {",
		"func reviewActionReason(summary *GenerationWorkQueueSummary) string {",
		"func dominantGenerationQualityGrade(summary *GenerationWorkQueueSummary) string {",
		"func blockingPlatformsForQueue(queue *GenerationWorkQueue) []string {",
		"func blockingQualityGradesForQueue(summary *GenerationWorkQueueSummary) []string {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func buildAssetGenerationOverview(queue *GenerationWorkQueue) *AssetGenerationOverview {",
		"func reviewActionKey(summary *GenerationWorkQueueSummary) string {",
		"func actionFiltersForKey(actionKey string, base *AssetGenerationRecommendedFilters) *AssetGenerationRecommendedFilters {",
	})
}
