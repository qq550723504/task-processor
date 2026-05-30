package listingkit

import "testing"

func TestTaskGenerationCurrentStateResultServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) getCurrentListingKitResult(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationCurrentStateSnapshotPhase(s).run(", 1)
	assertSourceOrderedContains(t, source, []string{
		"buildTaskGenerationCurrentStateSnapshotPhase(s).run(",
		"return snapshot.result, nil",
	})
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildTaskGenerationCurrentStateViewsPhase(",
		"buildActionPlatformRenderPreviews(",
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
		"task:    task,",
		"result:  result,",
		"tasks:   tasks,",
		"reviews: reviews,",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationCurrentStateViewsPhase(",
		"buildAssetGenerationOverview(",
		"buildActionPlatformRenderPreviews(",
		"getCurrentAssetGenerationQueue(",
		"getCurrentAssetGenerationOverview(",
		"getCurrentActionRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateOverviewServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) getCurrentAssetGenerationOverview(")

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
		"buildActionPlatformRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateQueueServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) getCurrentAssetGenerationQueue(")

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
		"buildGenerationQueuePage(",
		"buildActionPlatformRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateRenderPreviewsServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) getCurrentActionRenderPreviews(")

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
		"buildActionPlatformRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateViewsOverviewBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_current_state_views.go", "func (p *taskGenerationCurrentStateViewsPhase) overview(")

	assertSourceOccurrenceCount(t, source, "result.AssetGenerationOverview", 1)
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildActionPlatformRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateViewsQueueBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_current_state_views.go", "func (p *taskGenerationCurrentStateViewsPhase) queue(")

	assertSourceOccurrenceCount(t, source, "result.AssetGenerationQueue", 1)
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
		"buildActionPlatformRenderPreviews(",
	})
}

func TestTaskGenerationCurrentStateViewsRenderPreviewsBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_current_state_views.go", "func (p *taskGenerationCurrentStateViewsPhase) renderPreviews(")

	assertSourceOccurrenceCount(t, source, "buildActionPlatformRenderPreviews(result, query)", 1)
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"listGenerationReviews(",
		"withListingKitResultGenerationAndReview(",
	})
}
