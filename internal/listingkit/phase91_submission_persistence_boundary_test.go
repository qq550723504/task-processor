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

	directSource := readNamedFunctionSource(t, "task_direct_submission_support.go", "persistDirectSubmitSnapshot")
	assertSourceContainsAll(t, directSource, []string{
		"sheinpub.SetSubmissionSnapshot(",
	})
	assertSourceExcludesAll(t, directSource, []string{
		"setSheinSubmitSnapshot(",
	})

	stateSource := readNamedFunctionSource(t, "task_submission_state_service.go", "persistSuccessfulSheinDirectResponse")
	assertSourceContainsAll(t, stateSource, []string{
		"sheinpub.SetSubmissionRemoteResponse(",
	})
	assertSourceExcludesAll(t, stateSource, []string{
		"setSheinSubmitRemoteResponse(",
	})
}
