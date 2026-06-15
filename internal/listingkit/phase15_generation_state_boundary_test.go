package listingkit

import "testing"

func TestTaskGenerationCurrentStateResultServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service_support.go", "func (s *taskGenerationService) getCurrentListingKitResult(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationCurrentStateSnapshotPhase(s).run(", 1)
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildGenerationQueuePage(",
		"buildAssetGenerationOverview(",
		"buildTaskGenerationCurrentStateViewsPhase(",
		"buildActionPlatformRenderPreviews(",
		"AssetGenerationQueue",
		"AssetGenerationOverview",
	})
}

func TestTaskGenerationCurrentStateSnapshotPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_current_state_snapshot.go", "func (p *taskGenerationCurrentStateSnapshotPhase) run(")

	assertSourceOccurrenceCount(t, source, "p.service.repo.GetTask(", 1)
	assertSourceOccurrenceCount(t, source, "p.service.listAssetGenerationTasks(", 1)
	assertSourceOccurrenceCount(t, source, "p.service.listGenerationReviews(", 1)
	assertSourceOccurrenceCount(t, source, "withListingKitResultGenerationAndReview(", 1)
	assertSourceOrderedContains(t, source, []string{
		"p.service.repo.GetTask(",
		"p.service.listAssetGenerationTasks(",
		"p.service.listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
	})
	assertSourceContainsAll(t, source, []string{
		"taskGenerationCurrentStateSnapshot{",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationCurrentStateViewsPhase(",
		"buildGenerationQueuePage(",
		"buildAssetGenerationOverview(",
		"buildActionPlatformRenderPreviews(",
		"getCurrentAssetGenerationQueue(",
		"getCurrentAssetGenerationOverview(",
		"getCurrentActionRenderPreviews(",
		"AssetGenerationQueue",
		"AssetGenerationOverview",
	})
}

func TestTaskGenerationCurrentStateOverviewServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service_support.go", "func (s *taskGenerationService) getCurrentAssetGenerationOverview(")

	assertSourceOccurrenceCount(t, source, "s.getCurrentListingKitResult(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationCurrentStateViewsPhase().overview(", 1)
	assertSourceOrderedContains(t, source, []string{
		"s.getCurrentListingKitResult(",
		"buildTaskGenerationCurrentStateViewsPhase().overview(",
	})
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildAssetGenerationOverview(",
		"buildGenerationQueuePage(",
		"buildActionPlatformRenderPreviews(",
		"AssetGenerationQueue",
	})
}

func TestTaskGenerationCurrentStateQueueServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service_support.go", "func (s *taskGenerationService) getCurrentAssetGenerationQueue(")

	assertSourceOccurrenceCount(t, source, "s.getCurrentListingKitResult(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationCurrentStateViewsPhase().queue(", 1)
	assertSourceOrderedContains(t, source, []string{
		"s.getCurrentListingKitResult(",
		"buildTaskGenerationCurrentStateViewsPhase().queue(",
	})
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildAssetGenerationOverview(",
		"buildGenerationQueuePage(",
		"buildActionPlatformRenderPreviews(",
		"AssetGenerationOverview",
	})
}

func TestTaskGenerationCurrentStateRenderPreviewsServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service_support.go", "func (s *taskGenerationService) getCurrentActionRenderPreviews(")

	assertSourceOccurrenceCount(t, source, "s.getCurrentListingKitResult(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationCurrentStateViewsPhase().renderPreviews(", 1)
	assertSourceOrderedContains(t, source, []string{
		"s.getCurrentListingKitResult(",
		"buildTaskGenerationCurrentStateViewsPhase().renderPreviews(",
	})
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildAssetGenerationOverview(",
		"buildGenerationQueuePage(",
		"buildActionPlatformRenderPreviews(",
		"AssetGenerationOverview",
		"AssetGenerationQueue",
	})
}

func TestTaskGenerationCurrentStateViewsOverviewBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_current_state_views.go", "func (p *taskGenerationCurrentStateViewsPhase) overview(")

	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"AssetGenerationQueue",
		"buildGenerationQueuePage(",
		"buildActionPlatformRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateViewsQueueBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_current_state_views.go", "func (p *taskGenerationCurrentStateViewsPhase) queue(")

	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"AssetGenerationOverview",
		"buildAssetGenerationOverview(",
		"buildActionPlatformRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateViewsRenderPreviewsBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_current_state_views.go", "func (p *taskGenerationCurrentStateViewsPhase) renderPreviews(")

	assertSourceOccurrenceCount(t, source, "buildActionPlatformRenderPreviews(", 1)
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"AssetGenerationOverview",
		"AssetGenerationQueue",
		"buildAssetGenerationOverview(",
		"buildGenerationQueuePage(",
	})
}
