package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskStudioBatchGenerationOwnerBoundary(t *testing.T) {
	t.Parallel()

	serviceSrc, err := os.ReadFile("task_studio_batch_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"service.ensureServiceRunner()",
		"return s.serviceRunner.StartGeneration(ctx, batchID)",
		"return s.serviceRunner.PrepareGeneration(ctx, batchID)",
		"return s.serviceRunner.ResumeGeneration(ctx, batchID)",
		"return s.serviceRunner.RetryItems(ctx, batchID, req)",
		"return s.serviceRunner.PrepareRetryItems(ctx, batchID, req)",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should delegate through studio batch facade seam %q", needle)
		}
	}

	adapterSrc, err := os.ReadFile("task_studio_batch_generation_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_generation_adapter.go) error = %v", err)
	}
	adapterContent := string(adapterSrc)

	for _, needle := range []string{
		"type listingStudioBatchGenerationRunner = studiodomain.BatchGenerationService[",
		"func newListingStudioBatchGenerationService(s *taskStudioBatchService) *listingStudioBatchGenerationRunner {",
		"AdaptResumeResult: adaptCreateStudioBatchTasksResultToDetail,",
		"func adaptCreateStudioBatchTasksResultToDetail(result *CreateStudioBatchTasksResult) *StudioBatchDetail {",
	} {
		if !strings.Contains(adapterContent, needle) {
			t.Fatalf("task_studio_batch_generation_adapter.go should contain %q", needle)
		}
	}

	serviceAdapterSrc, err := os.ReadFile("task_studio_batch_service_adapter.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_service_adapter.go) error = %v", err)
	}
	serviceAdapterContent := string(serviceAdapterSrc)

	for _, needle := range []string{
		"type listingStudioBatchServiceRunner = studiodomain.BatchService[",
		"func newListingStudioBatchServiceRunner(s *taskStudioBatchService) *listingStudioBatchServiceRunner {",
		"StartGeneration: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {",
		"PrepareRetryItems: func(ctx context.Context, batchID string, itemIDs []string) (*StudioBatchDetail, error) {",
	} {
		if !strings.Contains(serviceAdapterContent, needle) {
			t.Fatalf("task_studio_batch_service_adapter.go should contain %q", needle)
		}
	}
}

func TestAdaptCreateStudioBatchTasksResultToDetailPreservesTaskOutcomes(t *testing.T) {
	t.Parallel()

	detail := adaptCreateStudioBatchTasksResultToDetail(&CreateStudioBatchTasksResult{
		CreatedTasks: []SheinStudioCreatedTask{{
			ID:       "created-task",
			DesignID: "created-design",
		}},
		ReusedTasks: []SheinStudioCreatedTask{{
			ID:         "reused-task",
			ReasonCode: "existing_task",
		}},
		RejectedTasks: []SheinStudioRejectedTask{{
			DesignID:   "rejected-design",
			ReasonCode: "baseline_missing",
		}},
		FailedTasks: []SheinStudioFailedTask{{
			DesignID: "failed-design",
			Message:  "submit failed",
		}},
	})

	if detail == nil {
		t.Fatal("detail is nil")
	}
	if len(detail.CreatedTasks) != 1 || detail.CreatedTasks[0].ID != "created-task" {
		t.Fatalf("created tasks = %+v, want created-task", detail.CreatedTasks)
	}
	if len(detail.ReusedTasks) != 1 || detail.ReusedTasks[0].ID != "reused-task" {
		t.Fatalf("reused tasks = %+v, want reused-task", detail.ReusedTasks)
	}
	if len(detail.RejectedTasks) != 1 || detail.RejectedTasks[0].ReasonCode != "baseline_missing" {
		t.Fatalf("rejected tasks = %+v, want baseline_missing", detail.RejectedTasks)
	}
	if len(detail.FailedTasks) != 1 || detail.FailedTasks[0].DesignID != "failed-design" {
		t.Fatalf("failed tasks = %+v, want failed-design", detail.FailedTasks)
	}
}
