package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskSubmissionRecoveryRouteSupportBoundary(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("task_submission_recovery_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_recovery_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func newTaskSubmissionRecoveryService(config taskSubmissionRecoveryServiceConfig) *taskSubmissionRecoveryService {",
		"func (s *taskSubmissionRecoveryService) acquireSheinSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {",
		"func (s *taskSubmissionRecoveryService) handleSheinWorkflowStartFailure(ctx context.Context, taskID string, task *Task, opts sheinWorkflowSubmitOptions, startErr error) error {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitRemote(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) executeRecoveredSheinSubmitRoute(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"type sheinSubmitStartedAtContextKey struct{}",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_submission_recovery_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) shouldRecoverLocally(state *sheinRecoveredRemoteState) bool {",
		"func (s *taskSubmissionRecoveryService) beginSheinSubmitLeaseWithoutStartTime(ctx context.Context, taskID, action, requestID string) (*Task, error) {",
		"func (s *taskSubmissionRecoveryService) buildSheinReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) buildMissingPackageAcquireError(error) error {",
		"func (s *taskSubmissionRecoveryService) recordWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
		"func (s *taskSubmissionRecoveryService) clearWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("task_submission_recovery_service.go should delegate route support helper %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_submission_recovery_service_route_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_recovery_service_route_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) shouldRecoverLocally(state *sheinRecoveredRemoteState) bool {",
		"func (s *taskSubmissionRecoveryService) beginSheinSubmitLeaseWithoutStartTime(ctx context.Context, taskID, action, requestID string) (*Task, error) {",
		"func (s *taskSubmissionRecoveryService) buildSheinReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) buildMissingPackageAcquireError(error) error {",
		"func (s *taskSubmissionRecoveryService) recordWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
		"func (s *taskSubmissionRecoveryService) clearWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_submission_recovery_service_route_support.go should contain %q", needle)
		}
	}
}
