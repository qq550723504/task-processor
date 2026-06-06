package listingkit

import "testing"

func TestSharedQueueQueryCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("shared_clone_home_owns_queue_query_clone", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
		})
	})

	t.Run("shared_clone_home_stays_separate_from_service_entrypoints", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, source, []string{
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, source, []string{
			"func ExecuteTaskGenerationAction(",
			"func cloneAssetGenerationActionTarget(",
		})
	})
}
