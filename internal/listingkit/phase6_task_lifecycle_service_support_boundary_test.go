package listingkit

import "testing"

func TestTaskLifecycleServiceSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "task_lifecycle_service.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func newTaskLifecycleService(",
		"func (s *taskLifecycleService) CreateGenerateTask(",
		"func (s *taskLifecycleService) GetTaskResult(",
		"func (s *taskLifecycleService) GetSDSBaselineReadiness(",
		"func (s *taskLifecycleService) ListTasks(",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func buildTaskListSummary(",
		"func incrementTaskListSummary(",
		"func pruneEmptyTaskListSummary(",
		"func (s *taskLifecycleService) prepareGenerateTask(",
		"func (s *taskLifecycleService) dispatchGenerateTask(",
		"func (s *taskLifecycleService) enqueueGenerateTask(",
		"func (s *taskLifecycleService) scheduleAsyncEnqueueRetry(",
		"func validateRequest(",
		"func validateSheinStudioAspectRatio(",
	})

	supportSource := readTaskGenerationSourceFile(t, "task_lifecycle_service_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func buildTaskListSummary(",
		"func incrementTaskListSummary(",
		"func pruneEmptyTaskListSummary(",
		"func (s *taskLifecycleService) prepareGenerateTask(",
		"func (s *taskLifecycleService) dispatchGenerateTask(",
		"func (s *taskLifecycleService) enqueueGenerateTask(",
		"func (s *taskLifecycleService) scheduleAsyncEnqueueRetry(",
		"func validateRequest(",
		"func validateSheinStudioAspectRatio(",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func newTaskLifecycleService(",
		"func (s *taskLifecycleService) CreateGenerateTask(",
		"func (s *taskLifecycleService) GetTaskResult(",
		"func (s *taskLifecycleService) GetSDSBaselineReadiness(",
		"func (s *taskLifecycleService) ListTasks(",
	})
}
