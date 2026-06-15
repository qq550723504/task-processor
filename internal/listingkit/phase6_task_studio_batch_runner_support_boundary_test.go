package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskStudioBatchRunnerSupportBoundary(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {",
		"func (s *taskStudioBatchService) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *taskStudioBatchService) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *taskStudioBatchService) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *taskStudioBatchService) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"func (s *taskStudioBatchService) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {",
		"func (s *taskStudioBatchService) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {",
		"func (s *taskStudioBatchService) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {",
		"func (s *taskStudioBatchService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
		"func (s *taskStudioBatchService) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *taskStudioBatchService) ensureDetailRunner() {",
		"func (s *taskStudioBatchService) ensureBatchRunner() {",
		"func (s *taskStudioBatchService) ensureReviewRunner() {",
		"func (s *taskStudioBatchService) ensureRetryRunner() {",
		"func (s *taskStudioBatchService) ensureTaskCreationRunner() {",
		"func (s *taskStudioBatchService) ensureTaskExecuteRunner() {",
		"func (s *taskStudioBatchService) ensureTaskPrepareRunner() {",
		"func (s *taskStudioBatchService) ensureTaskResumeRunner() {",
		"func (s *taskStudioBatchService) studioBatchSessionUpdater() func(context.Context, *SheinStudioSession) error {",
		"func (s *taskStudioBatchService) studioBatchUpdater() func(context.Context, *StudioBatchRecord) error {",
	} {
		if strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should delegate runner support helper %q", needle)
		}
	}

	supportSrc, err := os.ReadFile("task_studio_batch_runner_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_runner_support.go) error = %v", err)
	}
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) ensureDetailRunner() {",
		"func (s *taskStudioBatchService) ensureBatchRunner() {",
		"func (s *taskStudioBatchService) ensureReviewRunner() {",
		"func (s *taskStudioBatchService) ensureRetryRunner() {",
		"func (s *taskStudioBatchService) ensureTaskCreationRunner() {",
		"func (s *taskStudioBatchService) ensureTaskExecuteRunner() {",
		"func (s *taskStudioBatchService) ensureTaskPrepareRunner() {",
		"func (s *taskStudioBatchService) ensureTaskResumeRunner() {",
		"func (s *taskStudioBatchService) studioBatchSessionUpdater() func(context.Context, *SheinStudioSession) error {",
		"func (s *taskStudioBatchService) studioBatchUpdater() func(context.Context, *StudioBatchRecord) error {",
	} {
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_studio_batch_runner_support.go should contain %q", needle)
		}
	}
}
