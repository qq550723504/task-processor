package listingkit

import "testing"

func TestSheinSubmissionRecoveryStartFailureBoundary(t *testing.T) {
	t.Parallel()

	handlerSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "handleSheinWorkflowStartFailure")
	assertSourceContainsAll(t, handlerSource, []string{
		"return s.startFailureRunner.Handle(ctx, sheinWorkflowStartFailureInput{",
	})
	assertSourceExcludesAll(t, handlerSource, []string{
		"s.recordSubmissionFailure(",
		"s.clearSheinSubmitLeaseAfterStartFailure(",
		"errors.Join(",
	})

	serviceSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "newTaskSubmissionRecoveryService")
	assertSourceContainsAll(t, serviceSource, []string{
		"ClearFailure: func(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
		"return service.clearSheinSubmitLeaseAfterStartFailure(ctx, in.taskID, in.action, in.requestID, in.startErr)",
	})
	assertSourceExcludesAll(t, serviceSource, []string{
		"ClearFailure:  service.clearWorkflowStartFailure,",
	})

	recordSource := readNamedFunctionSource(t, "task_submission_recovery_service_route_support.go", "recordWorkflowStartFailure")
	assertSourceContainsAll(t, recordSource, []string{
		"s.recordSubmissionFailure(",
		"sheinpub.SubmissionPhaseValidate",
	})

	routeSupportSource := readTaskGenerationSourceFile(t, "task_submission_recovery_service_route_support.go")
	assertSourceExcludesAll(t, routeSupportSource, []string{
		"func (s *taskSubmissionRecoveryService) clearWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
	})
}
