package listingkit

import "testing"

func TestTaskGenerationActionRefreshDelegationBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_refresh.go", "func (p *taskGenerationActionRefreshPhase) run(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionRefreshExtractPhase(p.service).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionRefreshHydrationPhase()", 1)
	assertSourceOccurrenceCount(t, source, ".run(baseResult, refresh)", 1)
	assertSourceOrderedContains(t, source, []string{
		"buildTaskGenerationActionRefreshExtractPhase(p.service).run(",
		"buildTaskGenerationActionRefreshHydrationPhase()",
		".run(baseResult, refresh)",
	})
	assertSourceExcludesAll(t, source, []string{
		"getCurrentListingKitResult(",
		"AssetGenerationOverview",
		"buildActionPlatformRenderPreviews(",
		"PlatformAssetRenderPreviews =",
		"AssetRenderPreviews =",
	})
}

func TestTaskGenerationActionRefreshExtractOwnershipBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_refresh_extract.go", "func (p *taskGenerationActionRefreshExtractPhase) run(")

	assertSourceOccurrenceCount(t, source, "p.service.getCurrentListingKitResult(", 1)
	assertSourceOccurrenceCount(t, source, "buildActionPlatformRenderPreviews(", 1)
	assertSourceOrderedContains(t, source, []string{
		"p.service.getCurrentListingKitResult(",
		"buildActionPlatformRenderPreviews(",
		"taskGenerationActionRefreshExtractResult{",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationActionRefreshHydrationPhase(",
		"baseResult",
		"PlatformAssetRenderPreviews =",
		"AssetRenderPreviews =",
		"buildActionPlatformRenderPreviews(baseResult,",
	})
}

func TestTaskGenerationActionRefreshHydrationOwnershipBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_refresh_hydration.go", "func (p *taskGenerationActionRefreshHydrationPhase) run(")

	assertSourceOccurrenceCount(t, source, "buildActionPlatformRenderPreviews(baseResult, p.query)", 1)
	assertSourceOrderedContains(t, source, []string{
		"buildActionPlatformRenderPreviews(baseResult, p.query)",
		"PlatformAssetRenderPreviews =",
		"AssetRenderPreviews =",
		"taskGenerationActionRefreshResult{",
	})
	assertSourceExcludesAll(t, source, []string{
		"getCurrentListingKitResult(",
		"currentResult.AssetGenerationOverview",
		"buildTaskGenerationActionRefreshExtractPhase(",
		"p.service",
		"buildActionPlatformRenderPreviews(currentResult, query)",
	})
}
