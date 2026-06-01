package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskGenerationNavigationDispatchPlanServiceBoundary(t *testing.T) {
	t.Parallel()

	planSource := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) executeGenerationNavigationDispatchPlan(")
	assertSourceOccurrenceCount(t, planSource, "buildTaskGenerationNavigationDispatchPlanPhase(s).run(", 1)
	assertSourceExcludesAll(t, planSource, []string{
		"cloneGenerationNavigationDispatchPlan(",
		"generationNavigationDispatchPlanRunsInParallel(",
		"executeGenerationNavigationDispatchPlanParallel(",
		"executeGenerationNavigationDispatchPlanSequential(",
		"applyGenerationNavigationDispatchExecutionRules(",
		"generationNavigationDispatchStepDeduplicationKey(",
		"GetTaskGenerationQueue(",
		"GetTaskGenerationReviewPreview(",
		"GetTaskGenerationReviewSession(",
	})

	parallelSource := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) executeGenerationNavigationDispatchPlanParallel(")
	assertSourceOccurrenceCount(t, parallelSource, "buildTaskGenerationNavigationDispatchPlanParallelPhase(s).run(", 1)
	assertSourceExcludesAll(t, parallelSource, []string{
		"generationNavigationDispatchStepDeduplicationKey(",
		"generationNavigationDispatchPlanDeduplicatedStep(",
		"generationNavigationDispatchExecutionPendingStep(",
		"GetTaskGenerationQueue(",
		"GetTaskGenerationReviewPreview(",
		"GetTaskGenerationReviewSession(",
	})

	stepSource := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) executeGenerationNavigationDispatchPlanStep(")
	assertSourceOccurrenceCount(t, stepSource, "buildTaskGenerationNavigationDispatchStepExecutionPhase(s).run(", 1)
	assertSourceExcludesAll(t, stepSource, []string{
		"generationNavigationDispatchPlanRunsInParallel(",
		"cloneGenerationNavigationDispatchPlan(",
		"applyGenerationNavigationDispatchExecutionRules(",
		"applyExecutedPlanToDispatchResponse(",
		"finalizeGenerationReviewNavigationDispatchResponse(",
	})
}

func TestTaskGenerationNavigationDispatchPlanPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readNamedFunctionSource(t, "task_generation_navigation_dispatch_plan.go", "run")

	assertSourceContainsAll(t, source, []string{
		"cloneGenerationNavigationDispatchPlan(",
		"generationNavigationDispatchPlanRunsInParallel(",
		"executeGenerationNavigationDispatchPlanParallel(",
		"executeGenerationNavigationDispatchPlanSequential(",
		"applyGenerationNavigationDispatchExecutionRules(",
	})
	assertSourceExcludesAll(t, source, []string{
		"generationNavigationDispatchStepDeduplicationKey(",
		"generationNavigationDispatchPlanDeduplicatedStep(",
		"generationNavigationDispatchExecutionPendingStep(",
		"applyGenerationNavigationDispatchExecutionStats(",
		"GetTaskGenerationQueue(",
		"GetTaskGenerationReviewPreview(",
		"GetTaskGenerationReviewSession(",
		"applyExecutedPlanToDispatchResponse(",
		"finalizeGenerationReviewNavigationDispatchResponse(",
	})
}

func TestTaskGenerationNavigationDispatchPlanParallelPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_navigation_dispatch_plan_parallel.go")

	assertSourceContainsAll(t, source, []string{
		"buildEntries(",
		"maxParallelism(",
		"replayDeduplicatedSourceState(",
		"generationNavigationDispatchStepDeduplicationKey(",
		"generationNavigationDispatchPlanDeduplicatedStep(",
		"generationNavigationDispatchExecutionPendingStep(",
		"executeGenerationNavigationDispatchPlanStep(",
		"applyGenerationNavigationDispatchExecutionStats(",
	})
	assertSourceExcludesAll(t, source, []string{
		"GetTaskGenerationQueue(",
		"GetTaskGenerationReviewPreview(",
		"GetTaskGenerationReviewSession(",
		"cloneGenerationNavigationDispatchPlan(",
		"generationNavigationDispatchPlanRunsInParallel(",
		"applyGenerationNavigationDispatchExecutionRules(",
		"applyExecutedPlanToDispatchResponse(",
		"finalizeGenerationReviewNavigationDispatchResponse(",
	})
}

func TestTaskGenerationNavigationDispatchPlanStepExecutionPhaseBoundary(t *testing.T) {
	t.Parallel()

	source := readTaskGenerationSourceFile(t, "task_generation_navigation_dispatch_step_execution.go")

	assertSourceContainsAll(t, source, []string{
		"GetTaskGenerationQueue(",
		"GetTaskGenerationReviewPreview(",
		"GetTaskGenerationReviewSession(",
		"shouldStopGenerationNavigationDispatchPlan(",
		"generationNavigationDispatchPlanStopReason(",
		"generationNavigationDispatchPlanSkippedStep(",
		"queueReadState(",
		"previewReadState(",
		"sessionReadState(",
	})
	assertSourceExcludesAll(t, source, []string{
		"cloneGenerationNavigationDispatchPlan(",
		"generationNavigationDispatchPlanRunsInParallel(",
		"applyGenerationNavigationDispatchExecutionRules(",
		"applyExecutedPlanToDispatchResponse(",
		"finalizeGenerationReviewNavigationDispatchResponse(",
		"generationNavigationDispatchPlanDeduplicatedStep(",
		"generationNavigationDispatchExecutionPendingStep(",
	})
}

func readExactMethodSource(t *testing.T, path, signature string) string {
	t.Helper()

	sourceBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, signature)
	if start == -1 {
		t.Fatalf("%s should contain method signature %q", path, signature)
	}
	bodyStart := strings.Index(source[start:], "{")
	if bodyStart == -1 {
		t.Fatalf("%s should contain body for signature %q", path, signature)
	}
	bodyStart += start
	depth := 0
	for index := bodyStart; index < len(source); index++ {
		switch source[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : index+1]
			}
		}
	}
	t.Fatalf("%s should contain a complete body for signature %q", path, signature)
	return ""
}
