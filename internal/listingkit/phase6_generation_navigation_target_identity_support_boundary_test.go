package listingkit

import "testing"

func TestGenerationNavigationTargetIdentitySupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "generation_navigation_target_identity.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func applyIdentityToNavigationTarget(",
		"func cloneGenerationNavigationDescriptor(",
		"func cloneGenerationNavigationDispatchPlan(",
		"func applyGenerationNavigationDescriptorCloneShapePairing(",
		"func cloneGenerationNavigationFollowUpReadSlice(",
		"func cloneGenerationNavigationFollowUpRead(",
		"func applyGenerationNavigationFollowUpReadCloneShape(",
		"func cloneGenerationNavigationDispatchPlanStep(",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func buildGenerationNavigationDescriptor(",
		"func buildGenerationNavigationFollowUpReads(",
		"func buildGenerationNavigationDispatchPlan(",
		"func buildGenerationNavigationTargetCacheKey(",
		"func normalizeGenerationReviewDispatchKind(",
	})

	supportSource := readTaskGenerationSourceFile(t, "generation_navigation_target_identity_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func buildGenerationNavigationDescriptor(",
		"func buildGenerationNavigationFollowUpReads(",
		"func buildGenerationNavigationDispatchPlan(",
		"func buildGenerationNavigationTargetCacheKey(",
		"func normalizeGenerationReviewDispatchKind(",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func cloneGenerationNavigationDescriptor(",
		"func cloneGenerationNavigationDispatchPlan(",
		"func cloneGenerationNavigationFollowUpRead(",
		"func cloneGenerationNavigationDispatchPlanStep(",
	})
}
