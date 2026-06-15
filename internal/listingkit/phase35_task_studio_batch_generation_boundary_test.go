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
		"return s.batchRunner.StartGeneration(ctx, strings.TrimSpace(batchID))",
		"return s.batchRunner.PrepareGeneration(ctx, strings.TrimSpace(batchID))",
		"return s.batchRunner.ResumeGeneration(ctx, strings.TrimSpace(batchID))",
		"return s.batchRunner.RetryItems(ctx, normalizedBatchID, itemIDs)",
		"return s.retryRunner.PrepareRetryItems(ctx, normalizedBatchID, itemIDs)",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_studio_batch_service.go should delegate through batch runner seam %q", needle)
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
}
