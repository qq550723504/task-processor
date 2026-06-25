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

	runnerSource := readNamedFunctionSource(t, "task_temporal_submission_refresh_service.go", "newTaskTemporalSubmissionRefreshService")
	assertSourceContainsAll(t, runnerSource, []string{
		"FinishError: func(ctx context.Context, state *sheinTemporalRemoteRefreshState, remoteErr error) (*SheinRefreshRemoteStatusResult, error) {",
		"return nil, service.persistence.finishSheinTemporalRemoteRefreshFailure(ctx, state, remoteErr)",
		"FinishOK: func(ctx context.Context, state *sheinTemporalRemoteRefreshState) (*SheinRefreshRemoteStatusResult, error) {",
		"return service.persistence.finishSheinTemporalRemoteRefreshSuccess(ctx, state)",
	})
	assertSourceExcludesAll(t, runnerSource, []string{
		"FinishError:  service.finishTemporalRemoteRefreshError,",
		"FinishOK:     service.finishTemporalRemoteRefreshSuccess,",
	})
	refreshFileSource := readTaskGenerationSourceFile(t, "task_temporal_submission_refresh_service.go")
	assertSourceExcludesAll(t, refreshFileSource, []string{
		"func (s *taskTemporalSubmissionRefreshService) finishTemporalRemoteRefreshError(ctx context.Context, state *sheinTemporalRemoteRefreshState, remoteErr error) (*SheinRefreshRemoteStatusResult, error) {",
		"func (s *taskTemporalSubmissionRefreshService) finishTemporalRemoteRefreshSuccess(ctx context.Context, state *sheinTemporalRemoteRefreshState) (*SheinRefreshRemoteStatusResult, error) {",
	})

	persistenceSuccessSource := readNamedFunctionSource(t, "task_temporal_submission_persistence_service_support.go", "finishSheinTemporalRemoteRefreshSuccess")
	assertSourceContainsAll(t, persistenceSuccessSource, []string{
		"sheinpub.SubmissionStatePackage(",
	})
	assertSourceExcludesAll(t, persistenceSuccessSource, []string{
		"sheinpub.ResolveSubmissionRemoteRefreshSelection(",
	})
}
