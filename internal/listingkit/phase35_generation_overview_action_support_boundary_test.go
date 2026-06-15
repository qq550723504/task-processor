package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestGenerationOverviewSupportFilesOwnActionHelpers(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("generation_overview.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_overview.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func buildAssetGenerationOverview(queue *GenerationWorkQueue) *AssetGenerationOverview {",
		"func dominantGenerationQualityGrade(summary *GenerationWorkQueueSummary) string {",
		"func reviewActionKey(summary *GenerationWorkQueueSummary) string {",
		"func actionFiltersForKey(actionKey string, base *AssetGenerationRecommendedFilters) *AssetGenerationRecommendedFilters {",
		"func applyAssetGenerationActionFiltersMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {",
		"func applyAssetGenerationRegularActionKeyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("generation_overview.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildAssetGenerationActionTarget(queue *GenerationWorkQueue, actionKey string, filters *AssetGenerationRecommendedFilters) *AssetGenerationActionTarget {",
		"func buildAssetGenerationActionImpact(queue *GenerationWorkQueue, query *GenerationQueueQuery) *AssetGenerationActionImpact {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("generation_overview.go should delegate action helper %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("generation_overview_action_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_overview_action_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func buildAssetGenerationActionTarget(queue *GenerationWorkQueue, actionKey string, filters *AssetGenerationRecommendedFilters) *AssetGenerationActionTarget {",
		"func buildSecondaryActionTargets(queue *GenerationWorkQueue, actionKeys []string, filters *AssetGenerationRecommendedFilters) []*AssetGenerationActionTarget {",
		"func buildAssetGenerationActionImpact(queue *GenerationWorkQueue, query *GenerationQueueQuery) *AssetGenerationActionImpact {",
		"func actionInteractionMode(actionKey string) string {",
		"func buildPreviewCapabilitySecondaryActions(summary *GenerationWorkQueueSummary) ([]string, []string) {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("generation_overview_action_support.go should contain %q", needle)
		}
	}

	filterSupportSrc, err := os.ReadFile("generation_overview_filter_support.go")
	if err != nil {
		t.Fatalf("ReadFile(generation_overview_filter_support.go) error = %v", err)
	}
	filterSupportContent := string(filterSupportSrc)

	for _, needle := range []string{
		"func applyAssetGenerationPreviewCapabilityFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {",
		"func applyAssetGenerationIdealReviewFilters(filters *AssetGenerationRecommendedFilters) {",
		"func applyAssetGenerationReviewReadyFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {",
		"func applyAssetGenerationFailedRetryFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {",
		"func applyAssetGenerationProvisionalRetryFilterMutation(actionKey string, filters *AssetGenerationRecommendedFilters) bool {",
	} {
		if !strings.Contains(filterSupportContent, needle) {
			t.Fatalf("generation_overview_filter_support.go should contain %q", needle)
		}
	}
}
