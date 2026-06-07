package listingkit

import "testing"

func TestSharedRetryRequestSliceCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_request_shape_home_owns_both_slice_clones", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_clone_shape.go", "applyRetryGenerationTasksRequestCloneShape")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_clone_shape.go", "applyRetryGenerationTasksRequestCloneShape")

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

	t.Run("retry_request_taskid_clone_home_owns_only_taskid_slice_clone", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_taskid_clone.go", "applyRetryGenerationTasksRequestTaskIDClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_taskid_clone.go", "applyRetryGenerationTasksRequestTaskIDClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyRetryGenerationTasksRequestSlotClone",
		})
	})
}
