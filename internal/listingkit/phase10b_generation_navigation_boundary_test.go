package listingkit

import "testing"

func TestTaskGenerationNavigationDispatchServiceBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_service.go", "DispatchTaskGenerationNavigation")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationNavigationDispatchEntry().run(", 1)
	assertSourceOccurrenceCount(t, source, "dispatchGenerationNavigationPrimary(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationNavigationDispatchProjectionPhase()", 1)
	assertSourceOccurrenceCount(t, source, "executeGenerationNavigationDispatchPlan(", 1)
	assertSourceExcludesAll(t, source, []string{
		"normalizeGenerationReviewDispatchKind(",
		"resolveTaskGenerationNavigationPrimarySessionQuery(",
		"executeGenerationNavigationDispatchPlanSequential(",
		"executeGenerationNavigationDispatchPlanParallel(",
		"executeGenerationNavigationDispatchPlanStep(",
		"applyExecutedPlanToDispatchResponse(",
		"finalizeGenerationReviewNavigationDispatchResponse(",
	})
}

func TestTaskGenerationNavigationDispatchEntryBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_navigation_dispatch_entry.go", "run")

	assertSourceOccurrenceCount(t, source, "cloneGenerationReviewNavigationTarget(", 1)
	assertSourceOccurrenceCount(t, source, "ApplyGenerationConditionalBaselineToNavigationTarget(", 1)
	assertSourceOccurrenceCount(t, source, "normalizeGenerationActionResponseMode(", 1)
	assertSourceOccurrenceCount(t, source, "normalizeGenerationNavigationDispatchPlanMode(", 1)
	assertSourceExcludesAll(t, source, []string{
		"DispatchTaskGenerationNavigation(",
		"dispatchGenerationNavigationPrimary(",
		"executeGenerationNavigationDispatchPlan(",
		"applyExecutedPlanToDispatchResponse(",
		"finalizeGenerationReviewNavigationDispatchResponse(",
		"normalizeGenerationReviewDispatchKind(",
	})
}

func TestTaskGenerationNavigationDispatchPrimaryBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_navigation_dispatch_primary.go", "run")

	assertSourceOccurrenceCount(t, source, "normalizeGenerationReviewDispatchKind(", 1)
	assertSourceOccurrenceCount(t, source, "ExecuteTaskGenerationAction(", 1)
	assertSourceOccurrenceCount(t, source, "GetTaskGenerationReviewPreview(", 1)
	assertSourceOccurrenceCount(t, source, "GetTaskGenerationQueue(", 1)
	assertSourceOccurrenceCount(t, source, "GetTaskGenerationReviewSession(", 1)
	assertSourceOccurrenceCount(t, source, "resolveTaskGenerationNavigationPrimarySessionQuery(", 1)
	assertSourceExcludesAll(t, source, []string{
		"executeGenerationNavigationDispatchPlan(",
		"executeGenerationNavigationDispatchPlanSequential(",
		"executeGenerationNavigationDispatchPlanParallel(",
		"executeGenerationNavigationDispatchPlanStep(",
		"applyExecutedPlanToDispatchResponse(",
		"finalizeGenerationReviewNavigationDispatchResponse(",
	})
}

func TestTaskGenerationNavigationDispatchProjectionBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_navigation_dispatch_projection.go", "run")

	assertSourceOccurrenceCount(t, source, "response.PlanMode = planMode", 1)
	assertSourceOccurrenceCount(t, source, "applyExecutedPlanToDispatchResponse(", 1)
	assertSourceOccurrenceCount(t, source, "finalizeGenerationReviewNavigationDispatchResponse(", 1)
	assertSourceExcludesAll(t, source, []string{
		"normalizeGenerationReviewDispatchKind(",
		"ExecuteTaskGenerationAction(",
		"GetTaskGenerationReviewPreview(",
		"GetTaskGenerationQueue(",
		"GetTaskGenerationReviewSession(",
		"dispatchGenerationNavigationPrimary(",
		"executeGenerationNavigationDispatchPlan(",
		"executeGenerationNavigationDispatchPlanStep(",
	})
}
