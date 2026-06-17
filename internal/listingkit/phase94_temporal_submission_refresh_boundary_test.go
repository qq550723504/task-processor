package listingkit

import "testing"

func TestSheinTemporalSubmissionRefreshBoundary(t *testing.T) {
	t.Parallel()

	serviceSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "RefreshSheinPublishRemoteStatus")
	assertSourceContainsAll(t, serviceSource, []string{
		"return refresh.RefreshSheinPublishRemoteStatus(ctx, in)",
	})
	assertSourceExcludesAll(t, serviceSource, []string{
		"s.persistSheinSubmitPhase(",
		"s.refreshSheinSubmitRemoteStatus(",
		"ResolveSubmissionRemoteRefreshSelection(",
	})

	refreshSource := readNamedFunctionSource(t, "task_temporal_submission_refresh_service.go", "RefreshSheinPublishRemoteStatus")
	assertSourceContainsAll(t, refreshSource, []string{
		"buildSheinTemporalRemoteRefreshState(",
		"s.remoteRefreshRunner.Refresh(ctx, refreshState)",
	})

	stateSource := readNamedFunctionSource(t, "task_temporal_submission_refresh_service.go", "buildSheinTemporalRemoteRefreshState")
	assertSourceContainsAll(t, stateSource, []string{
		"sheinpub.SubmissionStartedAt(",
		"sheinpub.SubmissionResponseForAction(",
	})
	assertSourceExcludesAll(t, stateSource, []string{
		"sheinpub.ResolveSubmissionRemoteRefreshSelection(",
	})

	supportSource := readNamedFunctionSource(t, "task_temporal_submission_refresh_service.go", "persistTemporalRemoteRefreshPhase")
	assertSourceContainsAll(t, supportSource, []string{
		"s.persistSheinSubmitPhase(",
	})

	requestSource := readNamedFunctionSource(t, "task_temporal_submission_refresh_service.go", "buildTemporalRemoteRefreshRequest")
	assertSourceContainsAll(t, requestSource, []string{
		"s.buildSheinSubmitProductAPI(",
		"buildSheinRemoteRefreshRequest(",
	})

	failureSource := readNamedFunctionSource(t, "task_temporal_submission_refresh_service.go", "finishTemporalRemoteRefreshError")
	assertSourceContainsAll(t, failureSource, []string{
		"finishSheinTemporalRemoteRefreshFailure(",
	})

	successSource := readNamedFunctionSource(t, "task_temporal_submission_refresh_service.go", "finishTemporalRemoteRefreshSuccess")
	assertSourceContainsAll(t, successSource, []string{
		"finishSheinTemporalRemoteRefreshSuccess(",
	})

	persistenceSuccessSource := readNamedFunctionSource(t, "task_temporal_submission_persistence_service_support.go", "finishSheinTemporalRemoteRefreshSuccess")
	assertSourceContainsAll(t, persistenceSuccessSource, []string{
		"sheinpub.SubmissionStatePackage(",
	})
	assertSourceExcludesAll(t, persistenceSuccessSource, []string{
		"sheinpub.ResolveSubmissionRemoteRefreshSelection(",
	})
}
