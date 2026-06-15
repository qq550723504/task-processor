package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskSubmissionRecoveryServiceSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("task_submission_recovery_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_recovery_service.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func newTaskSubmissionRecoveryService(config taskSubmissionRecoveryServiceConfig) *taskSubmissionRecoveryService {",
		"func (s *taskSubmissionRecoveryService) acquireSheinSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) shouldRecoverLocally(state *sheinRecoveredRemoteState) bool {",
		"func (s *taskSubmissionRecoveryService) recordWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
		"type sheinSubmitStartedAtContextKey struct{}",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("task_submission_recovery_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildRecoveredSheinRemoteState(task *Task, action string) (*sheinRecoveredRemoteState, error) {",
		"func (s *taskSubmissionRecoveryService) persistRecoveredRemoteRefreshPhase(_ context.Context, state *sheinRecoveredRemoteState) error {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("task_submission_recovery_service.go should delegate helper family %q", needle)
		}
	}

	remoteSrc, err := os.ReadFile("task_submission_recovery_service_remote_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_recovery_service_remote_support.go) error = %v", err)
	}
	remoteContent := string(remoteSrc)

	for _, needle := range []string{
		"func buildRecoveredSheinRemoteState(task *Task, action string) (*sheinRecoveredRemoteState, error) {",
		"func (s *taskSubmissionRecoveryService) persistRecoveredRemoteRefreshPhase(_ context.Context, state *sheinRecoveredRemoteState) error {",
		"func (s *taskSubmissionRecoveryService) finishRecoveredRemoteRefreshSuccess(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
	} {
		if !strings.Contains(remoteContent, needle) {
			t.Fatalf("task_submission_recovery_service_remote_support.go should contain %q", needle)
		}
	}
}
