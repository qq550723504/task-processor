package listingkit

import "testing"

func TestSharedRetryRequestFinalCloneBoundary(t *testing.T) {
	t.Parallel()

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
			"applyRetryGenerationTasksRequestTaskIDSlotClonePairing",
		})
	})

	t.Run("retry_request_slot_clone_home_owns_only_slot_slice_clone", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_slot_clone.go", "applyRetryGenerationTasksRequestSlotClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_slot_clone.go", "applyRetryGenerationTasksRequestSlotClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyRetryGenerationTasksRequestTaskIDClone",
			"applyRetryGenerationTasksRequestTaskIDSlotClonePairing",
		})
	})
}
