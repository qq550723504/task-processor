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

	serviceSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "newTaskSubmissionRecoveryService")
	assertSourceContainsAll(t, serviceSource, []string{
		"BeginLease: func(ctx context.Context, taskID, action, requestID string) (*Task, error) {",
		"startedAt, _ := ctx.Value(sheinSubmitStartedAtContextKey{}).(time.Time)",
		"return service.beginSheinSubmitLease(ctx, taskID, action, requestID, startedAt)",
		"BuildReplayPreview: func(ctx context.Context, task *Task) (*ListingKitPreview, error) {",
		"return service.buildTaskPreview(ctx, task, \"shein\")",
		"BuildMissingPkgErr: func(error) error {",
		"return fmt.Errorf(\"%w: shein preview payload is not available\", ErrSubmitBlocked)",
	})
	assertSourceExcludesAll(t, serviceSource, []string{
		"BeginLease:       service.beginSheinSubmitLeaseWithoutStartTime,",
	})

	routeSupportSource := readTaskGenerationSourceFile(t, "task_submission_recovery_service_route_support.go")
	assertSourceExcludesAll(t, routeSupportSource, []string{
		"func (s *taskSubmissionRecoveryService) beginSheinSubmitLeaseWithoutStartTime(ctx context.Context, taskID, action, requestID string) (*Task, error) {",
		"func (s *taskSubmissionRecoveryService) buildSheinReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) buildMissingPackageAcquireError(error) error {",
	})
}
