package workspace

import "testing"

func TestBuildTaskWorkQueueUsesTaskWorkflowAndOverviewState(t *testing.T) {
	t.Parallel()

	if got := BuildTaskWorkQueue("processing", "", nil); got != WorkQueueGeneration {
		t.Fatalf("BuildTaskWorkQueue(processing) = %q, want %q", got, WorkQueueGeneration)
	}
	if got := BuildTaskWorkQueue("completed", WorkflowStatusPublished, nil); got != WorkQueuePublished {
		t.Fatalf("BuildTaskWorkQueue(published) = %q, want %q", got, WorkQueuePublished)
	}
	if got := BuildTaskWorkQueue("completed", "", &StatusOverview{Status: "blocked"}); got != WorkQueueRepair {
		t.Fatalf("BuildTaskWorkQueue(blocked) = %q, want %q", got, WorkQueueRepair)
	}
}

func TestBuildTaskActionQueuePrioritizesBlockingWarningAndReadyStates(t *testing.T) {
	t.Parallel()

	if got := BuildTaskActionQueue("completed", "", nil, []string{"category"}, nil); got != ActionQueueClassification {
		t.Fatalf("BuildTaskActionQueue(category) = %q, want %q", got, ActionQueueClassification)
	}
	if got := BuildTaskActionQueue("completed", "", nil, nil, []string{"manual_notes"}); got != ActionQueueManualReview {
		t.Fatalf("BuildTaskActionQueue(manual_notes) = %q, want %q", got, ActionQueueManualReview)
	}
	if got := BuildTaskActionQueue("completed", "", &StatusOverview{Status: "ready"}, nil, nil); got != ActionQueueSubmitReady {
		t.Fatalf("BuildTaskActionQueue(ready) = %q, want %q", got, ActionQueueSubmitReady)
	}
	if got := BuildTaskActionQueue("completed", WorkflowStatusPublished, &StatusOverview{Status: "ready"}, []string{"category"}, nil); got != "" {
		t.Fatalf("BuildTaskActionQueue(published) = %q, want empty", got)
	}
}
