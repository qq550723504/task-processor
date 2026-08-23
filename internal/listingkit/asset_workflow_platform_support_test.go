package listingkit

import (
	"testing"

	assetgeneration "task-processor/internal/asset/generation"
)

func TestPlatformGenerationTasksDoesNotRedispatchFailedTasks(t *testing.T) {
	t.Parallel()

	plan := &assetgeneration.Result{Tasks: []assetgeneration.Task{
		{ID: "shein-pending", Platform: "shein", ExecutionStatus: "planned"},
		{ID: "shein-failed", Platform: "shein", ExecutionStatus: "failed"},
		{ID: "amazon-failed", Platform: "amazon", ExecutionStatus: "failed"},
		{ID: "shein-completed", Platform: "shein", ExecutionStatus: "completed"},
	}}

	got := platformGenerationTasks("shein", plan)
	if len(got) != 1 || got[0].ID != "shein-pending" {
		t.Fatalf("platformGenerationTasks() = %+v, want only planned SHEIN task", got)
	}
}
