package generation

import "testing"

func TestCloneTasksDefensivelyCopiesMutableFields(t *testing.T) {
	t.Parallel()

	original := []Task{{
		ID:             "task-1",
		Lineage:        []string{"shein", "main"},
		SourceAssetIDs: []string{"source-1"},
		Metadata:       map[string]string{"slot": "main"},
	}}

	cloned := CloneTasks(original)
	if len(cloned) != 1 {
		t.Fatalf("CloneTasks() = %+v, want one task", cloned)
	}
	original[0].Lineage[0] = "mutated"
	original[0].SourceAssetIDs[0] = "mutated"
	original[0].Metadata["slot"] = "mutated"
	if cloned[0].Lineage[0] != "shein" || cloned[0].SourceAssetIDs[0] != "source-1" || cloned[0].Metadata["slot"] != "main" {
		t.Fatalf("CloneTasks() = %+v, want defensive copies", cloned)
	}
}

func TestMergeTasksReplacesExistingAndAppendsNewTasksInStableOrder(t *testing.T) {
	t.Parallel()

	existing := []Task{
		{ID: "keep", Status: "planned", Metadata: map[string]string{"slot": "old"}},
		{ID: "replace", Status: "planned"},
	}
	updates := []Task{
		{ID: "replace", Status: "completed"},
		{ID: "new", Status: "completed"},
	}

	merged := MergeTasks(existing, updates)
	if len(merged) != 3 {
		t.Fatalf("MergeTasks() = %+v, want 3 tasks", merged)
	}
	if merged[0].ID != "keep" || merged[1].ID != "replace" || merged[2].ID != "new" {
		t.Fatalf("MergeTasks() order = %+v, want existing order followed by new updates", merged)
	}
	if merged[1].Status != "completed" {
		t.Fatalf("MergeTasks() replaced task = %+v, want update value", merged[1])
	}
	existing[0].Metadata["slot"] = "mutated"
	if merged[0].Metadata["slot"] != "old" {
		t.Fatalf("MergeTasks() = %+v, want defensive copy of existing tasks", merged)
	}
}

func TestCloneTasksReturnsNilForEmptyInput(t *testing.T) {
	t.Parallel()

	if cloned := CloneTasks(nil); cloned != nil {
		t.Fatalf("CloneTasks(nil) = %+v, want nil", cloned)
	}
}
