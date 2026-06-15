package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskStudioBatchTaskCreationOwnerBoundary(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
		"return s.taskPrepareRunner.PrepareTaskCreation(ctx, normalizedBatchID, listingStudioBatchTaskPrepareState{",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should contain %q", needle)
		}
	}

	flowSrc, err := os.ReadFile("task_studio_batch_task_flow_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_flow_support.go) error = %v", err)
	}
	flowContent := string(flowSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) resumeStudioBatchTaskCreation(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {",
		"s.ensureTaskResumeRunner()",
		"return s.taskCreationRunner.ResumeTaskCreation(ctx, batchID)",
	} {
		if !strings.Contains(flowContent, needle) {
			t.Fatalf("task_studio_batch_task_flow_support.go should contain %q", needle)
		}
	}

	adapterSrc, err := os.ReadFile("task_studio_batch_task_creation_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_creation_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"type listingStudioBatchTaskCreationRunner = studiodomain.BatchTaskCreationService[",
		"func newListingStudioBatchTaskCreationService(s *taskStudioBatchService) *listingStudioBatchTaskCreationRunner {",
		"PrepareTaskCreation: func(ctx context.Context, batchID string, state studiodomain.BatchTaskPrepareState[SheinStudioSession, StudioBatchRecord]) (*CreateStudioBatchTasksResult, error) {",
		"FinalizeTaskCreation: func(ctx context.Context, batchID string, state studiodomain.BatchTaskResumeFinalizeState[SheinStudioSession, StudioBatchRecord, SheinStudioCreatedTask, SheinStudioFailedTask]) (*CreateStudioBatchTasksResult, error) {",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_task_creation_adapter.go should contain %q", needle)
		}
	}
}
