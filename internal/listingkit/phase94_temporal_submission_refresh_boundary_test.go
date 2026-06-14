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
		"s.persistSheinSubmitPhase(",
		"s.refreshSheinSubmitRemoteStatus(",
		"ResolveSubmissionRemoteRefreshSelection(",
		"finishSheinTemporalRemoteRefreshFailure(",
		"finishSheinTemporalRemoteRefreshSuccess(",
	})
}
