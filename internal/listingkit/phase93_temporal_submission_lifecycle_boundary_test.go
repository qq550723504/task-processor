package listingkit

import "testing"

func TestSheinTemporalSubmissionLifecycleBoundary(t *testing.T) {
	t.Parallel()

	beginSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "BeginSheinPublishAttempt")
	assertSourceContainsAll(t, beginSource, []string{
		"return temporal.BeginSheinPublishAttempt(ctx, in)",
	})
	assertSourceExcludesAll(t, beginSource, []string{
		"s.beginSheinSubmitLease(",
		"errSheinSubmitReplayExisting",
	})

	validateSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "ValidateSheinPublishReadiness")
	assertSourceContainsAll(t, validateSource, []string{
		"return temporal.ValidateSheinPublishReadiness(ctx, in)",
	})
	assertSourceExcludesAll(t, validateSource, []string{
		"prepareSheinSubmitReadinessForAction(",
		"validateSheinSubmitReadinessGates(",
	})

	lifecycleSource := readNamedFunctionSource(t, "task_temporal_submission_lifecycle_service.go", "ValidateSheinPublishReadiness")
	assertSourceContainsAll(t, lifecycleSource, []string{
		"s.loadSheinPreparedPublishState(",
		"buildSheinSubmitReadinessWithPodForAction(",
		"validateSheinSubmitReadinessGates(",
	})
}
