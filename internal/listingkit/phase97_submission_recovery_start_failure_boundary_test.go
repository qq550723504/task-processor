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

	recordSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "recordWorkflowStartFailure")
	assertSourceContainsAll(t, recordSource, []string{
		"s.recordSubmissionFailure(",
		"sheinpub.SubmissionPhaseValidate",
	})

	clearSource := readNamedFunctionSource(t, "task_submission_recovery_service.go", "clearWorkflowStartFailure")
	assertSourceContainsAll(t, clearSource, []string{
		"s.clearSheinSubmitLeaseAfterStartFailure(",
	})
}
