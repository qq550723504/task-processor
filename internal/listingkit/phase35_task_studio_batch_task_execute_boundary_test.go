package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskStudioBatchTaskExecuteOwnerBoundary(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {",
		"return s.serviceRunner.CreateTasks(ctx, batchID, req)",
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
		"func (s *taskStudioBatchService) completeStudioBatchTaskExecution(",
		"func (s *taskStudioBatchService) prepareStudioBatchTaskExecuteCandidates(",
	} {
		if !strings.Contains(flowContent, needle) {
			t.Fatalf("task_studio_batch_task_flow_support.go should contain %q", needle)
		}
	}

	adapterSrc, err := os.ReadFile("task_studio_batch_task_execute_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_execute_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"type listingStudioBatchTaskExecuteRunner = studiodomain.BatchTaskExecuteService[",
		"func newListingStudioBatchTaskExecuteService(s *taskStudioBatchService) *listingStudioBatchTaskExecuteRunner {",
		"LoadItems: func(ctx context.Context, batchID string, designIDs []string) ([]listingStudioBatchTaskExecuteCandidate, error) {",
		"Finalize: func(ctx context.Context, batchID string, session *SheinStudioSession, created []SheinStudioCreatedTask, failed []SheinStudioFailedTask) (*CreateStudioBatchTasksResult, error) {",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_task_execute_adapter.go should contain %q", needle)
		}
	}

	serviceAdapterSrc, err := os.ReadFile("task_studio_batch_service_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service_adapter.go) error = %v", err)
	}
	serviceAdapterContent := string(serviceAdapterSrc)

	for _, needle := range []string{
		"CreateTasks: func(ctx context.Context, batchID string, designIDs []string) (*CreateStudioBatchTasksResult, error) {",
		"return s.taskExecuteRunner.Execute(ctx, batchID, designIDs)",
	} {
		if !strings.Contains(serviceAdapterContent, needle) {
			t.Fatalf("task_studio_batch_service_adapter.go should contain %q", needle)
		}
	}
}
