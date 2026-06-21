package listingkit

import "testing"

func TestBuildStudioBatchStatusGroupsClassifiesMixedBatchDetail(t *testing.T) {
	t.Parallel()

	groups := BuildStudioBatchStatusGroups(&StudioBatchDetail{
		Items: []StudioBatchItemDetail{
			{Item: StudioBatchItemRecord{ID: "item-ready", Status: StudioBatchItemStatusReviewReady}},
			{Item: StudioBatchItemRecord{ID: "item-fix", Status: StudioBatchItemStatusReviewReady}, Designs: []StudioMaterializedDesignRecord{
				{ID: "design-rejected", ReviewStatus: StudioMaterializedDesignReviewStatusRejected},
			}},
			{Item: StudioBatchItemRecord{ID: "item-running", Status: StudioBatchItemStatusGenerating}},
			{Item: StudioBatchItemRecord{ID: "item-failed", Status: StudioBatchItemStatusFailed}},
		},
		CreatedTasks: []SheinStudioCreatedTask{
			{ID: "task-draft", DesignID: "design-draft", Title: "Draft saved", Status: "draft_saved"},
			{ID: "task-published", DesignID: "design-published", Title: "Published"},
		},
		FailedTasks: []SheinStudioFailedTask{
			{DesignID: "design-submit-failed", Title: "Submit failed", Message: "quota exceeded"},
		},
	})

	assertStudioBatchStatusGroup(t, groups, "submittable", 1, "item-ready")
	assertStudioBatchStatusGroup(t, groups, "needs_fix", 1, "item-fix")
	assertStudioBatchStatusGroup(t, groups, "processing", 1, "item-running")
	assertStudioBatchStatusGroup(t, groups, "generation_failed", 1, "item-failed")
	assertStudioBatchStatusGroup(t, groups, "submission_failed", 1, "design-submit-failed")
	assertStudioBatchStatusGroup(t, groups, "draft_saved", 1, "task-draft")
	assertStudioBatchStatusGroup(t, groups, "published", 1, "task-published")
}

func TestBuildStudioBatchStatusGroups_UsesExplicitCreatedTaskState(t *testing.T) {
	detail := &StudioBatchDetail{
		CreatedTasks: []SheinStudioCreatedTask{
			{ID: "task-1", Title: "Style 1", DesignID: "design-1", Status: "task_created"},
			{ID: "task-2", Title: "Style 2", DesignID: "design-2", Status: "draft_saved"},
			{ID: "task-3", Title: "Style 3", DesignID: "design-3", Status: "published"},
		},
	}

	groups := BuildStudioBatchStatusGroups(detail)
	if got := groups.ByKey["task_created"].Count; got != 1 {
		t.Fatalf("task_created count = %d, want 1", got)
	}
	if got := groups.ByKey["task_created"].IDs; len(got) != 1 || got[0] != "task-1" {
		t.Fatalf("task_created ids = %#v, want [task-1]", got)
	}
	if got := groups.ByKey["draft_saved"].Count; got != 1 {
		t.Fatalf("draft_saved count = %d, want 1", got)
	}
	if got := groups.ByKey["draft_saved"].IDs; len(got) != 1 || got[0] != "task-2" {
		t.Fatalf("draft_saved ids = %#v, want [task-2]", got)
	}
	if got := groups.ByKey["published"].Count; got != 1 {
		t.Fatalf("published count = %d, want 1", got)
	}
	if got := groups.ByKey["published"].IDs; len(got) != 1 || got[0] != "task-3" {
		t.Fatalf("published ids = %#v, want [task-3]", got)
	}
}

func TestBuildStudioBatchStatusGroups_DoesNotTreatEmptyCreatedTaskStateAsDraftSaved(t *testing.T) {
	detail := &StudioBatchDetail{
		CreatedTasks: []SheinStudioCreatedTask{
			{ID: "task-1", Title: "Style 1", DesignID: "design-1"},
		},
	}

	groups := BuildStudioBatchStatusGroups(detail)
	if _, ok := groups.ByKey["draft_saved"]; ok {
		t.Fatalf("draft_saved group should not be present for an empty created task state: %#v", groups.ByKey["draft_saved"])
	}
	if got := groups.ByKey["task_created"].Count; got != 1 {
		t.Fatalf("task_created count = %d, want 1", got)
	}
}

func assertStudioBatchStatusGroup(t *testing.T, groups StudioBatchStatusGroups, key string, count int, id string) {
	t.Helper()
	group, ok := groups.ByKey[key]
	if !ok {
		t.Fatalf("missing group %q in %#v", key, groups)
	}
	if group.Count != count {
		t.Fatalf("%s count = %d, want %d", key, group.Count, count)
	}
	for _, got := range group.IDs {
		if got == id {
			return
		}
	}
	t.Fatalf("%s ids = %#v, want %q", key, group.IDs, id)
}
