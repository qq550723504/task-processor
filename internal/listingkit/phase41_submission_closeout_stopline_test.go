package listingkit

import "testing"

func TestSubmissionCloseoutStopLineBoundary(t *testing.T) {
	t.Parallel()

	t.Run("submit_state_file_is_publishing_transition_adapter", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "shein_submit_state.go", "completeSheinSubmitAttemptAt")
		callNames := readNamedFunctionCallNames(t, "shein_submit_state.go", "completeSheinSubmitAttemptAt")

		assertSourceContainsAll(t, source, []string{
			"return sheinpub.CompleteSubmitAttemptAt(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"CompleteSubmitAttemptAt",
		})
	})

	t.Run("refresh_service_remains_runner_adapter_over_submission_and_publishing_seams", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_submission_refresh_service.go", "newTaskSubmissionRefreshService")
		assertSourceContainsAll(t, source, []string{
			"submissiondomain.NewStatusRefreshService(",
			"LoadState:           svc.loadSheinSubmissionRefreshState,",
			"ResolveConfirmation: svc.resolveSubmissionRefreshConfirmation,",
			"Finish:              svc.finishSubmissionRefresh,",
		})
	})

	t.Run("temporal_persistence_support_remains_runner_backed_root_callback_layer", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_temporal_submission_persistence_service_support.go", "persistSheinTemporalSubmissionSuccess")
		callNames := readNamedFunctionCallNames(t, "task_temporal_submission_persistence_service_support.go", "persistSheinTemporalSubmissionSuccess")

		assertSourceContainsAll(t, source, []string{
			"state.completion.startedAt = sheinpub.SubmissionStartedAt(",
			"return s.resultRunner.PersistSuccess(ctx, submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"SubmissionStartedAt",
			"PersistSuccess",
		})
	})
}
