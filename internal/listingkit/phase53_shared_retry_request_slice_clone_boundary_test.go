package listingkit

import "testing"

func TestSharedRetryRequestSliceCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_request_shape_home_routes_through_slice_clone_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_clone_shape.go", "applyRetryGenerationTasksRequestCloneShape")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_clone_shape.go", "applyRetryGenerationTasksRequestCloneShape")

		assertSourceContainsAll(t, source, []string{
			"applyRetryGenerationTasksRequestSliceClone(req, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyRetryGenerationTasksRequestSliceClone",
		})
	})

	t.Run("retry_request_slice_clone_home_owns_taskids_and_slots_clone", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_slice_clone.go", "applyRetryGenerationTasksRequestSliceClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_slice_clone.go", "applyRetryGenerationTasksRequestSliceClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
		})
	})
}
