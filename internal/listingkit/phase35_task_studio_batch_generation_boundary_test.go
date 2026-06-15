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
