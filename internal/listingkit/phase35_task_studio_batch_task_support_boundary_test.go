package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskStudioBatchTaskSupportFilesOwnSeparatedHelperFamilies(t *testing.T) {
	t.Parallel()

	creationSrc, err := os.ReadFile("task_studio_batch_task_creation_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_creation_support.go) error = %v", err)
	}
	creationContent := string(creationSrc)

	for _, needle := range []string{
		"func shouldResumeStudioBatchTaskCreation(ctx context.Context, repo StudioBatchRepository, batchID string) bool {",
	} {
		if !strings.Contains(creationContent, needle) {
			t.Fatalf("task_studio_batch_task_creation_support.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *taskStudioBatchService) findExistingStudioBatchTask(",
		"func buildStudioBatchTaskGenerateRequest(",
	} {
		if strings.Contains(creationContent, needle) {
			t.Fatalf("task_studio_batch_task_creation_support.go should delegate helper family %q", needle)
		}
	}

	existingSrc, err := os.ReadFile("task_studio_batch_task_existing_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_existing_support.go) error = %v", err)
	}
	existingContent := string(existingSrc)

	for _, needle := range []string{
		"func (s *taskStudioBatchService) findExistingStudioBatchTask(",
		"func studioBatchTaskMatchesSelection(",
		"func mergeStudioCreatedTasks(",
	} {
		if !strings.Contains(existingContent, needle) {
			t.Fatalf("task_studio_batch_task_existing_support.go should contain %q", needle)
		}
	}

	requestSrc, err := os.ReadFile("task_studio_batch_task_request_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_studio_batch_task_request_support.go) error = %v", err)
	}
	requestContent := string(requestSrc)

	for _, needle := range []string{
		"func buildStudioBatchTaskGenerateRequest(",
		"func buildStudioBatchTaskSDSOptions(",
		"func buildStudioBatchTaskVariantOptions(",
		"func toGenerateRequestSelectedSDSImages(",
		"func parseStudioBatchTaskStoreID(raw string) int64 {",
		"func buildStudioBatchTaskStyleID(designID string) string {",
	} {
		if !strings.Contains(requestContent, needle) {
			t.Fatalf("task_studio_batch_task_request_support.go should contain %q", needle)
		}
	}
}
