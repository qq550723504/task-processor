package listingkit

import (
	"testing"

	assetgeneration "task-processor/internal/asset/generation"
)

func TestBuildAssetGenerationProjectionBuildsSharedBundle(t *testing.T) {
	t.Parallel()

	tasks := []assetgeneration.Task{
		{
			TaskID:          "task-asset-projection-1",
			ID:              "shein:shein-main-model",
			Platform:        "shein",
			RecipeID:        "shein-main-model",
			Slot:            "main",
			ExecutionMode:   assetgeneration.ExecutionModeDeferredStub,
			ExecutionStatus: "completed",
			CanExecute:      true,
		},
	}

	projection := buildAssetGenerationProjection(nil, tasks)
	if projection == nil {
		t.Fatal("projection = nil")
	}
	if projection.Summary == nil {
		t.Fatal("summary = nil")
	}
	if projection.Queue == nil {
		t.Fatal("queue = nil")
	}
	if projection.Overview == nil {
		t.Fatal("overview = nil")
	}
	if len(projection.Tasks) != 1 {
		t.Fatalf("tasks = %+v", projection.Tasks)
	}

	tasks[0].Platform = "amazon"
	if projection.Tasks[0].Platform != "shein" {
		t.Fatalf("projection tasks mutated with source slice: %+v", projection.Tasks)
	}
}
