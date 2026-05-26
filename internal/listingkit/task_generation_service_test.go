package listingkit

import (
	"context"
	"testing"
	"time"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
)

func TestTaskGenerationServiceGetTaskGenerationTasksAppliesFilters(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	task := &Task{
		ID:        "task-generation-service-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Result:    &ListingKitResult{TaskID: "task-generation-service-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	var listCalls int
	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo: repo,
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			listCalls++
			return []assetgeneration.Task{
				{TaskID: taskID, ID: "amazon:amazon-lifestyle", Platform: "amazon", Slot: "auxiliary", ExecutionMode: assetgeneration.ExecutionModeRendererBacked, ExecutionStatus: "completed", SatisfiedBy: assetgeneration.ExecutionModeGeneratedAsset, CanExecute: true},
				{TaskID: taskID, ID: "shein:shein-main-model", Platform: "shein", Slot: "main", ExecutionMode: assetgeneration.ExecutionModeDeferredStub, ExecutionStatus: "completed", SatisfiedBy: "fallback_asset", CanExecute: true},
			}, nil
		},
	})

	page, err := generation.GetTaskGenerationTasks(context.Background(), task.ID, &GenerationTaskQuery{
		Platform:    "shein",
		Slot:        "main",
		SatisfiedBy: "fallback_asset",
	})
	if err != nil {
		t.Fatalf("GetTaskGenerationTasks() error = %v", err)
	}
	if listCalls != 1 {
		t.Fatalf("list calls = %d, want 1", listCalls)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].ID != "shein:shein-main-model" {
		t.Fatalf("tasks = %+v, want filtered shein main task", page.Tasks)
	}
	if page.Summary == nil || page.Summary.FallbackTasks != 1 {
		t.Fatalf("summary = %+v, want fallback summary", page.Summary)
	}
}

func TestTaskGenerationServiceRetryTaskGenerationTasksReturnsEmptyPageWithoutSelection(t *testing.T) {
	t.Parallel()

	repo := &stubGenerationRepo{}
	assetRepository := assetrepo.NewMemRepository()
	task := &Task{
		ID:        "task-generation-retry-service-1",
		Status:    TaskStatusCompleted,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request:   &GenerateRequest{Platforms: []string{"shein"}},
		Result:    &ListingKitResult{TaskID: "task-generation-retry-service-1"},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := assetRepository.SaveInventory(context.Background(), &asset.Inventory{
		Ref: asset.InventoryRef{TaskID: task.ID},
	}); err != nil {
		t.Fatalf("SaveInventory() error = %v", err)
	}

	generator := &stubWorkflowAssetGenerator{
		dispatchResult: &assetgeneration.Result{},
		dispatchErrAt: map[int]error{
			1: context.Canceled,
		},
	}
	generation := newTaskGenerationService(taskGenerationServiceConfig{
		repo:           repo,
		assetRepo:      assetRepository,
		assetGenerator: generator,
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return []assetgeneration.Task{}, nil
		},
		buildRetryGenerationTaskSelection: func(ctx context.Context, task *Task, inventory *asset.Inventory, existing []assetgeneration.Task, req *RetryGenerationTasksRequest) ([]assetgeneration.Task, error) {
			return nil, nil
		},
	})

	page, err := generation.RetryTaskGenerationTasks(context.Background(), task.ID, &RetryGenerationTasksRequest{})
	if err != nil {
		t.Fatalf("RetryTaskGenerationTasks() error = %v", err)
	}
	if generator.dispatchCalls != 0 {
		t.Fatalf("dispatch calls = %d, want 0", generator.dispatchCalls)
	}
	if page == nil || page.Total != 0 || page.MatchedQueue == nil || page.ExecutedQueue == nil {
		t.Fatalf("page = %+v, want empty retry page with queue placeholders", page)
	}
}
