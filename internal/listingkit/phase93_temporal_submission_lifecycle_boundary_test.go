package listingkit

import (
	"os"
	"testing"
)

func TestSheinTemporalSubmissionLifecycleBoundary(t *testing.T) {
	t.Parallel()

	beginSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "BeginSheinPublishAttempt")
	assertSourceContainsAll(t, beginSource, []string{
		"return lifecycle.BeginSheinPublishAttempt(ctx, in)",
	})
	assertSourceExcludesAll(t, beginSource, []string{
		"s.beginSheinSubmitLease(",
		"errSheinSubmitReplayExisting",
	})

	validateSource := readNamedFunctionSource(t, "service_shein_publish_temporal_entrypoints.go", "ValidateSheinPublishReadiness")
	assertSourceContainsAll(t, validateSource, []string{
		"return lifecycle.ValidateSheinPublishReadiness(ctx, in)",
	})
	assertSourceExcludesAll(t, validateSource, []string{
		"prepareSheinSubmitReadinessForAction(",
		"validateSheinSubmitReadinessGates(",
	})

	lifecycleSource := readNamedFunctionSource(t, "task_temporal_submission_lifecycle_service.go", "ValidateSheinPublishReadiness")
	assertSourceContainsAll(t, lifecycleSource, []string{
		"loadSheinTemporalPreparedPublishState(ctx, in, s.loadSheinPublishTask, s.normalizeSheinSubmitPackage)",
		"buildSheinSubmitReadinessWithPodForAction(",
		"validateSheinSubmitReadinessGates(",
	})
	assertSourceExcludesAll(t, lifecycleSource, []string{
		"s.loadSheinPublishTaskState",
	})
	lifecycleServiceSrc, err := os.ReadFile("task_temporal_submission_lifecycle_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_temporal_submission_lifecycle_service.go) error = %v", err)
	}
	assertSourceExcludesAll(t, string(lifecycleServiceSrc), []string{
		"func (s *taskTemporalSubmissionLifecycleService) loadSheinPublishTaskState(ctx context.Context, taskID string) (*Task, *SheinPackage, error) {",
	})
}
