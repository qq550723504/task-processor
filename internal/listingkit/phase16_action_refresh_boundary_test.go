package listingkit

import "testing"

func TestTaskGenerationActionRefreshDelegationBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_refresh.go", "func (p *taskGenerationActionRefreshPhase) run(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionRefreshExtractPhase(p.service).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationActionRefreshHydrationPhase()", 1)
	assertSourceOccurrenceCount(t, source, "hydration.run(baseResult, refresh)", 1)
	assertSourceOrderedContains(t, source, []string{
		"buildTaskGenerationActionRefreshExtractPhase(p.service).run(",
		"buildTaskGenerationActionRefreshHydrationPhase()",
		"hydration.query = query",
		"hydration.run(baseResult, refresh)",
	})
	assertSourceExcludesAll(t, source, []string{
		"getCurrentListingKitResult(",
		"AssetGenerationOverview",
		"buildActionPlatformRenderPreviews(",
		"PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil),",
		"AssetRenderPreviews = append([]AssetRenderPreview(nil),",
	})
}

func TestTaskGenerationActionRefreshExtractOwnershipBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_refresh_extract.go", "func (p *taskGenerationActionRefreshExtractPhase) run(")

	assertSourceOccurrenceCount(t, source, "p.service.getCurrentListingKitResult(", 1)
	assertSourceOccurrenceCount(t, source, "currentResult.AssetGenerationOverview", 1)
	assertSourceOccurrenceCount(t, source, "buildActionPlatformRenderPreviews(currentResult, query)", 1)
	assertSourceOrderedContains(t, source, []string{
		"p.service.getCurrentListingKitResult(",
		"currentResult.AssetGenerationOverview",
		"buildActionPlatformRenderPreviews(currentResult, query)",
		"taskGenerationActionRefreshExtractResult{",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationActionRefreshHydrationPhase(",
		"buildActionPlatformRenderPreviews(baseResult, p.query)",
		"PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil),",
		"AssetRenderPreviews = append([]AssetRenderPreview(nil),",
		"baseResult",
	})
}

func TestTaskGenerationActionRefreshHydrationOwnershipBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_action_refresh_hydration.go", "func (p *taskGenerationActionRefreshHydrationPhase) run(")

	assertSourceOccurrenceCount(t, source, "buildActionPlatformRenderPreviews(baseResult, p.query)", 1)
	assertSourceOccurrenceCount(t, source, "currentResult.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), platformRenderPreviews...)", 1)
	assertSourceOccurrenceCount(t, source, "currentResult.AssetRenderPreviews = append([]AssetRenderPreview(nil), baseResult.AssetRenderPreviews...)", 1)
	assertSourceOrderedContains(t, source, []string{
		"platformRenderPreviews := refresh.platformRenderPreviews",
		"buildActionPlatformRenderPreviews(baseResult, p.query)",
		"currentResult.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), platformRenderPreviews...)",
		"currentResult.AssetRenderPreviews = append([]AssetRenderPreview(nil), baseResult.AssetRenderPreviews...)",
		"taskGenerationActionRefreshResult{",
	})
	assertSourceExcludesAll(t, source, []string{
		"getCurrentListingKitResult(",
		"currentResult.AssetGenerationOverview",
		"buildTaskGenerationActionRefreshExtractPhase(",
		"p.service",
	})
}
