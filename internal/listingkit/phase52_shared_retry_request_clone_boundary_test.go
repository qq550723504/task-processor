package listingkit

import "testing"

func TestSharedRetryRequestCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_request_clone_home_owns_top_level_copy_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_shared_clone.go", "cloneRetryGenerationTasksRequest")
		callNames := readNamedFunctionCallNames(t, "task_generation_shared_clone.go", "cloneRetryGenerationTasksRequest")

		assertSourceContainsAll(t, source, []string{
			"cloned := *req",
			"applyRetryGenerationTasksRequestCloneShape(req, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyRetryGenerationTasksRequestCloneShape",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{})
	})

	t.Run("retry_request_clone_home_owns_both_slice_clones", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_shared_clone.go", "applyRetryGenerationTasksRequestCloneShape")
		callNames := readNamedFunctionCallNames(t, "task_generation_shared_clone.go", "applyRetryGenerationTasksRequestCloneShape")

		assertSourceContainsAll(t, source, []string{
			"applyRetryGenerationTasksRequestTaskIDClone(req, cloned)",
			"applyRetryGenerationTasksRequestSlotClone(req, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyRetryGenerationTasksRequestTaskIDClone",
			"applyRetryGenerationTasksRequestSlotClone",
		})
	})
}
