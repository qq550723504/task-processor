package listingkit

import (
	"context"
	"testing"
	"time"

	assetgeneration "task-processor/internal/asset/generation"
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
