package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskSubmissionStatePersistenceSupportBoundary(t *testing.T) {
	t.Parallel()

	stateSrc, err := os.ReadFile("task_submission_state_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_state_service.go) error = %v", err)
	}
	stateContent := string(stateSrc)

	for _, needle := range []string{
		"func newTaskSubmissionStateService(config taskSubmissionStateServiceConfig) *taskSubmissionStateService {",
		"func (s *taskSubmissionStateService) persistSuccessfulSheinSubmission(ctx context.Context, taskID string, task *Task, action string) error {",
		"func (s *taskSubmissionStateService) persistSheinDirectSubmitPhase(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, phase string) error {",
		"func (s *taskSubmissionStateService) persistSheinSubmitPhase(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string) error {",
		"func (s *taskSubmissionStateService) persistSuccessfulSheinDirectResponse(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, supplierCode string, response *sheinpub.SubmissionResponse) error {",
	} {
		if !strings.Contains(stateContent, needle) {
			t.Fatalf("task_submission_state_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *taskSubmissionStateService) finishSheinDirectSubmitAttempt(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, response *sheinpub.SubmissionResponse, responseErr error) error {",
		"func (s *taskSubmissionStateService) completeDirectSubmitAttempt(",
		"func (s *taskSubmissionStateService) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {",
		"func (s *taskSubmissionStateService) recordSheinSubmissionFailureForState(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestedID, phase string, submitErr error) error {",
		"func (s *taskSubmissionStateService) prepareDirectFailurePersistence(",
		"func (s *taskSubmissionStateService) persistDirectSuccessFallback(",
		"func (s *taskSubmissionStateService) persistDirectFailureFallback(",
		"func (s *taskSubmissionStateService) failSheinDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {",
		"func (s *taskSubmissionStateService) recordFailureState(",
	} {
		if strings.Contains(stateContent, needle) {
			t.Fatalf("task_submission_state_service.go should delegate persistence support helper %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_submission_state_persistence_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_state_persistence_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskSubmissionStateService) finishSheinDirectSubmitAttempt(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, response *sheinpub.SubmissionResponse, responseErr error) error {",
		"func (s *taskSubmissionStateService) completeDirectSubmitAttempt(",
		"func (s *taskSubmissionStateService) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {",
		"func (s *taskSubmissionStateService) recordSheinSubmissionFailureForState(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestedID, phase string, submitErr error) error {",
		"func (s *taskSubmissionStateService) prepareDirectFailurePersistence(",
		"func (s *taskSubmissionStateService) persistDirectSuccessFallback(",
		"func (s *taskSubmissionStateService) persistDirectFailureFallback(",
		"func (s *taskSubmissionStateService) failSheinDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {",
		"func (s *taskSubmissionStateService) recordFailureState(",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_submission_state_persistence_support.go should contain %q", needle)
		}
	}
}
