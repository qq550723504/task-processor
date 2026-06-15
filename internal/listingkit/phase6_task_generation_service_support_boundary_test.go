package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestTaskGenerationServiceSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSrc, err := os.ReadFile("task_generation_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_generation_service.go) error = %v", err)
	}
	supportSrc, err := os.ReadFile("task_generation_service_support.go")
	if err != nil {
		t.Fatalf("ReadFile(task_generation_service_support.go) error = %v", err)
	}

	rootContent := string(rootSrc)
	supportContent := string(supportSrc)

	for _, needle := range []string{
		"func newTaskGenerationService(config taskGenerationServiceConfig) *taskGenerationService {",
		"func (s *taskGenerationService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {",
		"func (s *taskGenerationService) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error) {",
		"func (s *taskGenerationService) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {",
		"func (s *taskGenerationService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {",
	} {
		if !strings.Contains(rootContent, needle) {
			t.Fatalf("task_generation_service.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func (s *taskGenerationService) executeLayerTemporalAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (bool, *GenerationActionExecutionResult, error) {",
		"func (s *taskGenerationService) getCurrentAssetGenerationOverview(ctx context.Context, taskID string) (*AssetGenerationOverview, error) {",
		"func (s *taskGenerationService) getCurrentListingKitResult(ctx context.Context, taskID string) (*ListingKitResult, error) {",
		"func (s *taskGenerationService) dispatchGenerationNavigationPrimary(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationReviewNavigationDispatchResponse, error) {",
		"func (s *taskGenerationService) executeGenerationNavigationDispatchPlanStep(ctx context.Context, taskID string, step GenerationNavigationDispatchStep, responseMode string) *GenerationNavigationDispatchExecutionStep {",
	} {
		if strings.Contains(rootContent, needle) {
			t.Fatalf("task_generation_service.go should delegate helper %q", needle)
		}
		if !strings.Contains(supportContent, needle) {
			t.Fatalf("task_generation_service_support.go should contain %q", needle)
		}
	}
}
