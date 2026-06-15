package listingkit

import "testing"

func TestSheinSubmissionRecoveryBoundary(t *testing.T) {
	t.Parallel()

	recoverSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "executeRecoveredSheinSubmitRoute")
	assertSourceContainsAll(t, recoverSource, []string{
		"return s.recoveryRouteRunner.Recover(ctx, state)",
	})
	assertSourceExcludesAll(t, recoverSource, []string{
		"SubmissionResponseAcceptedForAction(",
		"recoverSheinSubmitLocally(",
		"recoverSheinSubmitViaRemoteConfirmation(",
	})

	localRouteSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "shouldRecoverLocally")
	assertSourceContainsAll(t, localRouteSource, []string{
		"sheinpub.SubmissionResponseAcceptedForAction(",
	})

	localSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "recoverSheinSubmitLocally")
	assertSourceContainsAll(t, localSource, []string{
		"persistSheinRemoteCompletionSuccess(",
		"finalizeRecoveredSheinSubmission(",
	})

	remoteSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "recoverSheinSubmitViaRemoteConfirmation")
	assertSourceContainsAll(t, remoteSource, []string{
		"return s.remoteRefreshRunner.Refresh(ctx, state)",
	})
}
