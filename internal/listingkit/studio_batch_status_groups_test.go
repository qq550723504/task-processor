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
			{ID: "task-draft", DesignID: "design-draft", Title: "Draft saved"},
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
