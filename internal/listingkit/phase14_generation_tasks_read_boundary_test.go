package listingkit

import "testing"

func TestTaskGenerationTasksReadServiceDelegationBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_service.go", "func (s *taskGenerationService) GetTaskGenerationTasks(")

	assertSourceOccurrenceCount(t, source, "buildTaskGenerationTasksReadSnapshotPhase(s).run(", 1)
	assertSourceOccurrenceCount(t, source, "buildTaskGenerationTasksReadPagePhase().run(", 1)
	assertSourceOrderedContains(t, source, []string{
		"buildTaskGenerationTasksReadSnapshotPhase(s).run(",
		"buildTaskGenerationTasksReadPagePhase().run(",
	})
	assertSourceExcludesAll(t, source, []string{
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"filterGenerationTasks(",
		"sortGenerationTasks(",
		"paginateGenerationTasks(",
		"buildGenerationTaskPage(",
		"getCurrentListingKitResult(",
		"withListingKitResultGenerationAndReview(",
	})
}

func TestTaskGenerationTasksReadSnapshotOwnershipBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_tasks_read_snapshot.go", "func (p *taskGenerationTasksReadSnapshotPhase) run(")

	assertSourceOccurrenceCount(t, source, "p.service.repo.GetTask(", 1)
	assertSourceOccurrenceCount(t, source, "p.service.listAssetGenerationTasks(", 1)
	assertSourceOrderedContains(t, source, []string{
		"p.service.repo.GetTask(",
		"p.service.listAssetGenerationTasks(",
	})
	assertSourceContainsAll(t, source, []string{
		"taskGenerationTasksReadSnapshot{",
		"task:  task,",
		"tasks: tasks,",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationTasksReadPagePhase(",
		"filterGenerationTasks(",
		"sortGenerationTasks(",
		"paginateGenerationTasks(",
		"buildGenerationTaskPage(",
		"getCurrentListingKitResult(",
		"withListingKitResultGenerationAndReview(",
	})
}

func TestTaskGenerationTasksReadPageOwnershipBoundary(t *testing.T) {
	t.Parallel()

	source := readExactMethodSource(t, "task_generation_tasks_read_page.go", "func (p *taskGenerationTasksReadPagePhase) run(")

	assertSourceOccurrenceCount(t, source, "filterGenerationTasks(", 1)
	assertSourceOccurrenceCount(t, source, "sortGenerationTasks(", 1)
	assertSourceOccurrenceCount(t, source, "paginateGenerationTasks(", 1)
	assertSourceOccurrenceCount(t, source, "buildGenerationTaskPage(", 1)
	assertSourceOrderedContains(t, source, []string{
		"filterGenerationTasks(",
		"sortGenerationTasks(",
		"paginateGenerationTasks(",
		"buildGenerationTaskPage(",
	})
	assertSourceContainsAll(t, source, []string{
		"snapshot.task.ID",
		"snapshot.task.UpdatedAt",
	})
	assertSourceExcludesAll(t, source, []string{
		"buildTaskGenerationTasksReadSnapshotPhase(",
		"repo.GetTask(",
		"listAssetGenerationTasks(",
		"getCurrentListingKitResult(",
		"withListingKitResultGenerationAndReview(",
	})
}
