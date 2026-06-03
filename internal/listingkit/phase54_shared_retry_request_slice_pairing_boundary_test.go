package listingkit

import "testing"

func TestSharedRetryRequestSlicePairingBoundary(t *testing.T) {
	t.Parallel()

	t.Run("retry_request_slice_home_routes_through_taskid_slot_pairing_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_slice_clone.go", "applyRetryGenerationTasksRequestSliceClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_slice_clone.go", "applyRetryGenerationTasksRequestSliceClone")

		assertSourceContainsAll(t, source, []string{
			"applyRetryGenerationTasksRequestTaskIDSlotClonePairing(req, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.TaskIDs = append([]string(nil), req.TaskIDs...)",
			"cloned.Slots = append([]string(nil), req.Slots...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyRetryGenerationTasksRequestTaskIDSlotClonePairing",
		})
	})

	t.Run("retry_request_taskid_slot_pairing_home_owns_both_slice_clones", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_retry_request_taskid_slot_clone_pairing.go", "applyRetryGenerationTasksRequestTaskIDSlotClonePairing")
		callNames := readNamedFunctionCallNames(t, "task_generation_retry_request_taskid_slot_clone_pairing.go", "applyRetryGenerationTasksRequestTaskIDSlotClonePairing")

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
