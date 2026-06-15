package listingkit

import "testing"

func TestGenerationRecoverySummarySupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "generation_recovery_summary.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func buildGenerationRecoverySummaryFromQueue(queue *GenerationWorkQueue) *GenerationRecoverySummary {",
		"func buildGenerationRecoverySummaryFromDescriptors(items []GenerationPanelResourceDescriptor) *GenerationRecoverySummary {",
		"func applyGenerationRecoverySummaryToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {",
		"func applyGenerationRecoverySummaryToReviewSessionResponse(response *GenerationReviewSessionResponse) *GenerationReviewSessionResponse {",
		"func applyGenerationRecoverySummaryToReviewPreviewResponse(response *GenerationReviewPreviewResponse) *GenerationReviewPreviewResponse {",
		"func applyGenerationRecoverySummaryToActionResult(result *GenerationActionExecutionResult) *GenerationActionExecutionResult {",
		"func applyGenerationRecoveryArbitrationToOverview(overview *AssetGenerationOverview) *AssetGenerationOverview {",
		"func finalizeGenerationOverviewActionSummary(overview *AssetGenerationOverview) *AssetGenerationOverview {",
		"func applyGenerationRecoveryArbitrationToPlatformCard(card *ListingKitPlatformCard) {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func cloneGenerationRecoverySummary(summary *GenerationRecoverySummary) *GenerationRecoverySummary {",
		"func selectGenerationPanelRecoveryDescriptors(items []GenerationPanelResourceDescriptor) (*GenerationPanelResourceDescriptor, []GenerationPanelResourceDescriptor) {",
		"func applyGenerationPanelResourceRecovery(item *GenerationPanelResourceDescriptor) {",
		"func generationRecoveryProfileForHint(hint string) generationRecoveryProfile {",
		"func shouldPreferRecoveryAsPrimaryCTA(primaryActionKey string, summary *GenerationRecoverySummary) bool {",
	})

	supportSource := readTaskGenerationSourceFile(t, "generation_recovery_summary_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func cloneGenerationRecoverySummary(summary *GenerationRecoverySummary) *GenerationRecoverySummary {",
		"func selectGenerationPanelRecoveryDescriptors(items []GenerationPanelResourceDescriptor) (*GenerationPanelResourceDescriptor, []GenerationPanelResourceDescriptor) {",
		"func applyGenerationPanelResourceRecovery(item *GenerationPanelResourceDescriptor) {",
		"func generationRecoveryProfileForHint(hint string) generationRecoveryProfile {",
		"func shouldPreferRecoveryAsPrimaryCTA(primaryActionKey string, summary *GenerationRecoverySummary) bool {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func buildGenerationRecoverySummaryFromQueue(queue *GenerationWorkQueue) *GenerationRecoverySummary {",
		"func buildGenerationRecoverySummaryFromDescriptors(items []GenerationPanelResourceDescriptor) *GenerationRecoverySummary {",
		"func applyGenerationRecoverySummaryToQueuePage(page *GenerationQueuePage) *GenerationQueuePage {",
		"func applyGenerationRecoveryArbitrationToOverview(overview *AssetGenerationOverview) *AssetGenerationOverview {",
	})
}
