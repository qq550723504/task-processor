package listingkit

import "testing"

func TestSheinSubmissionPersistenceBoundary(t *testing.T) {
	t.Parallel()

	temporalSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "PersistSheinPublishSuccess")
	assertSourceContainsAll(t, temporalSource, []string{
		"return persistence.PersistSheinPublishSuccess(ctx, in)",
	})
	assertSourceExcludesAll(t, temporalSource, []string{
		"sheinpub.ApplySubmissionPersistenceInput(",
	})

	serviceSource := readNamedFunctionSource(t, "task_temporal_submission_persistence_service.go", "PersistSheinPublishSuccess")
	assertSourceContainsAll(t, serviceSource, []string{
		"loadSheinSubmitPersistenceState(",
		"return s.persistSheinTemporalSubmissionSuccess(",
	})

	loaderSource := readNamedFunctionSource(t, "task_temporal_submission_persistence_service_support.go", "loadSheinSubmitPersistenceState")
	assertSourceContainsAll(t, loaderSource, []string{
		"sheinpub.ApplySubmissionPersistenceInput(",
	})

	directSource := readNamedFunctionSource(t, "task_direct_submission_support.go", "newSheinDirectSubmitPayloadStages")
	assertSourceContainsAll(t, directSource, []string{
		"PersistSnapshot: func(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {",
		"sheinpub.SetSubmissionSnapshot(",
	})
	assertSourceExcludesAll(t, directSource, []string{
		"PersistSnapshot:        s.persistDirectSubmitSnapshot,",
		"setSheinSubmitSnapshot(",
	})
	directSupportSource := readTaskGenerationSourceFile(t, "task_direct_submission_support.go")
	assertSourceExcludesAll(t, directSupportSource, []string{
		"func (s *taskDirectSubmissionService) persistDirectSubmitSnapshot(_ context.Context, in submissiondomain.PayloadStageContext[*Task, *SheinPackage], snapshot *sheinpub.SubmitSnapshot) error {",
	})

	stateSource := readNamedFunctionSource(t, "task_submission_state_service.go", "persistSuccessfulSheinDirectResponse")
	assertSourceContainsAll(t, stateSource, []string{
		"sheinpub.SetSubmissionRemoteResponse(",
	})
	assertSourceExcludesAll(t, stateSource, []string{
		"setSheinSubmitRemoteResponse(",
	})

	remoteRefreshSource := readNamedFunctionSource(t, "task_temporal_submission_persistence_service_support.go", "confirmedTemporalRemoteRefreshResponse")
	assertSourceContainsAll(t, remoteRefreshSource, []string{
		"sheinmarketpub.ConfirmedSubmissionMessage(",
	})
	assertSourceExcludesAll(t, remoteRefreshSource, []string{
		"sheinpub.ConfirmedSubmissionResponse(",
	})
}
