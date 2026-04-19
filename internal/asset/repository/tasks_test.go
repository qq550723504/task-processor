package repository_test

import (
	"context"
	"testing"

	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
)

func TestMemRepositorySavesAndListsGenerationTasks(t *testing.T) {
	t.Parallel()

	repo := assetrepo.NewMemRepository()
	tasks := []assetgeneration.Task{
		{
			ID:              "shein:shein-main-model",
			Platform:        "shein",
			RecipeID:        "shein-main-model",
			Status:          "planned",
			ExecutionStatus: "planned",
			ExecutionMode:   "deferred_generation",
			CanExecute:      true,
		},
		{
			ID:              "amazon:amazon-selling-point",
			Platform:        "amazon",
			RecipeID:        "amazon-selling-point",
			Status:          "planned",
			ExecutionStatus: "planned",
			ExecutionMode:   "deferred_generation",
			CanExecute:      true,
		},
	}

	if err := repo.SaveGenerationTasks(context.Background(), "task-1", tasks); err != nil {
		t.Fatalf("SaveGenerationTasks() error = %v", err)
	}
	items, err := repo.ListGenerationTasks(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 tasks", items)
	}
	if items[0].TaskID != "task-1" {
		t.Fatalf("first item = %+v, want task_id propagated", items[0])
	}
	if items[0].ExecutionMode == "" {
		t.Fatalf("first item = %+v, want execution mode", items[0])
	}
}
