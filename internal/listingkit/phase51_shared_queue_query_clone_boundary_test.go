package listingkit

import "testing"

func TestSharedQueueQueryCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("queue_query_clone_home_owns_only_queue_query_clone", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "generation_queue_query_clone.go")

		assertSourceContainsAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneRetryGenerationTasksRequest(",
			"func ExecuteTaskGenerationAction(",
			"func cloneAssetGenerationActionTarget(",
		})
	})

	t.Run("retry_request_clone_stays_in_shared_retry_home", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "task_generation_shared_clone.go")

		assertSourceContainsAll(t, source, []string{
			"func cloneRetryGenerationTasksRequest(",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
			"func ExecuteTaskGenerationAction(",
			"func cloneAssetGenerationActionTarget(",
		})
	})
}
