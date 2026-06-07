package listingkit

import "testing"

func TestSharedRetryRequestSlicePairingBoundary(t *testing.T) {
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

	t.Run("retry_request_slot_clone_home_owns_only_slot_slice_clone", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_clone_shape.go", "applyRetryGenerationTasksRequestSlotClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_clone_shape.go", "applyRetryGenerationTasksRequestSlotClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyRetryGenerationTasksRequestTaskIDClone",
		})
	})
}
