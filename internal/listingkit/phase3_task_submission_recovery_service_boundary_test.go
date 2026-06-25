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
		"func (s *taskSubmissionRecoveryService) handleSheinWorkflowStartFailure(ctx context.Context, taskID string, task *Task, opts sheinWorkflowSubmitOptions, startErr error) error {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitRemote(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {",
		"return s.recoveryRouteRunner.Recover(ctx, recoveredState)",
		"type sheinSubmitStartedAtContextKey struct{}",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("task_submission_recovery_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func buildRecoveredSheinRemoteState(task *Task, action string) (*sheinRecoveredRemoteState, error) {",
		"func (s *taskSubmissionRecoveryService) persistRecoveredRemoteRefreshPhase(_ context.Context, state *sheinRecoveredRemoteState) error {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) recordWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
		"func (s *taskSubmissionRecoveryService) executeRecoveredSheinSubmitRoute(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
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
		"func (s *taskSubmissionRecoveryService) completeSheinRecoveredRemoteSuccess(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
	} {
		if !strings.Contains(remoteContent, needle) {
			t.Fatalf("task_submission_recovery_service_remote_support.go should contain %q", needle)
		}
	}
	if strings.Contains(remoteContent, "func (s *taskSubmissionRecoveryService) finishRecoveredRemoteRefreshSuccess(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {") {
		t.Fatal("task_submission_recovery_service_remote_support.go should not keep thin recovered remote refresh success wrapper; use completeSheinRecoveredRemoteSuccess directly")
	}
	if strings.Contains(remoteContent, "func (s *taskSubmissionRecoveryService) finishRecoveredRemoteRefreshError(ctx context.Context, state *sheinRecoveredRemoteState, remoteErr error) (*ListingKitPreview, error) {") {
		t.Fatal("task_submission_recovery_service_remote_support.go should not keep thin recovered remote refresh error wrapper; wire persistSheinRecoveredRemoteFailure inline")
	}
	if strings.Contains(remoteContent, "func buildRecoveredSheinRemoteRefreshState(state *sheinRecoveredRemoteState) *sheinRemoteRefreshExecutionState {") {
		t.Fatal("task_submission_recovery_service_remote_support.go should not keep thin recovered remote refresh state wrapper; build the request from recovered state directly")
	}

	routeSrc, err := os.ReadFile("task_submission_recovery_service_route_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_recovery_service_route_support.go) error = %v", err)
	}
	routeContent := string(routeSrc)

	for _, needle := range []string{
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {",
		"func (s *taskSubmissionRecoveryService) shouldRecoverLocally(state *sheinRecoveredRemoteState) bool {",
		"func (s *taskSubmissionRecoveryService) recordWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {",
	} {
		if !strings.Contains(routeContent, needle) {
			t.Fatalf("task_submission_recovery_service_route_support.go should contain %q", needle)
		}
	}
	if strings.Contains(routeContent, "func (s *taskSubmissionRecoveryService) buildSheinReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {") {
		t.Fatal("task_submission_recovery_service_route_support.go should not keep thin replay preview wrapper; wire the lease callback inline")
	}
	if strings.Contains(routeContent, "func (s *taskSubmissionRecoveryService) buildMissingPackageAcquireError(error) error {") {
		t.Fatal("task_submission_recovery_service_route_support.go should not keep thin missing-package error wrapper; wire the lease callback inline")
	}
	if strings.Contains(routeContent, "func (s *taskSubmissionRecoveryService) clearWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {") {
		t.Fatal("task_submission_recovery_service_route_support.go should not keep thin workflow-start clear wrapper; wire the start-failure callback inline")
	}
	if strings.Contains(routeContent, "func (s *taskSubmissionRecoveryService) beginSheinSubmitLeaseWithoutStartTime(ctx context.Context, taskID, action, requestID string) (*Task, error) {") {
		t.Fatal("task_submission_recovery_service_route_support.go should not keep thin lease-start wrapper; wire BeginLease inline")
	}
}
