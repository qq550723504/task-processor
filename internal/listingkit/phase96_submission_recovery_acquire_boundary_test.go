package listingkit

import "testing"

func TestSheinSubmissionRecoveryAcquireBoundary(t *testing.T) {
	t.Parallel()

	acquireSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "acquireSheinSubmitTask")
	assertSourceContainsAll(t, acquireSource, []string{
		"return s.leaseAcquireRunner.Acquire(",
		"sheinSubmitStartedAtContextKey{}",
	})
	assertSourceExcludesAll(t, acquireSource, []string{
		"errSheinSubmitReplayExisting",
		"errSheinSubmitRecoverRemote",
		"errSheinSubmitMissingPackage",
		"recoverSheinSubmitRemote(",
		"buildTaskPreview(",
	})

	beginSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "beginSheinSubmitLeaseWithoutStartTime")
	assertSourceContainsAll(t, beginSource, []string{
		"ctx.Value(sheinSubmitStartedAtContextKey{})",
		"beginSheinSubmitLease(",
	})

	replaySource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "buildSheinReplayPreview")
	assertSourceContainsAll(t, replaySource, []string{
		"s.buildTaskPreview(ctx, task, \"shein\")",
	})

	missingSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "buildMissingPackageAcquireError")
	assertSourceContainsAll(t, missingSource, []string{
		"ErrSubmitBlocked",
		"shein preview payload is not available",
	})
}
