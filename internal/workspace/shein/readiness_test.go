package shein

import "testing"

func TestSubmitChecklistGroupForKey(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"category":         "required",
		"attribute_review": "required",
		"request_draft":    "recommended",
		"manual_notes":     "optional",
	}

	for key, want := range cases {
		key, want := key, want
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			if got := SubmitChecklistGroupForKey(key); got != want {
				t.Fatalf("SubmitChecklistGroupForKey(%q) = %q, want %q", key, got, want)
			}
		})
	}
}

func TestJoinReadinessLabels(t *testing.T) {
	t.Parallel()

	items := []ReadinessItem[string, string]{
		{Label: "类目骨架"},
		{Label: ""},
		{Label: "主图资产"},
	}

	if got := JoinReadinessLabels(items, "、"); got != "类目骨架、主图资产" {
		t.Fatalf("JoinReadinessLabels() = %q", got)
	}
}

func TestChecklistItemCount(t *testing.T) {
	t.Parallel()

	checklist := &SubmitChecklist[string, string]{
		Required:    []ChecklistGroupItem[string, string]{{Key: "category"}},
		Recommended: []ChecklistGroupItem[string, string]{{Key: "request_draft"}},
		Optional:    []ChecklistGroupItem[string, string]{{Key: "manual_notes"}},
	}

	if got := ChecklistItemCount(checklist); got != 3 {
		t.Fatalf("ChecklistItemCount() = %d, want 3", got)
	}
}

func TestCloneReadinessItems(t *testing.T) {
	t.Parallel()

	items := []ReadinessItem[string, string]{{Key: "category", Label: "类目骨架"}}
	cloned := CloneReadinessItems(items)
	if len(cloned) != 1 || cloned[0].Key != "category" {
		t.Fatalf("CloneReadinessItems() = %+v", cloned)
	}
	if &cloned[0] == &items[0] {
		t.Fatal("CloneReadinessItems() should return a new slice")
	}
}

func TestFindKeys(t *testing.T) {
	t.Parallel()

	items := []ReadinessItem[string, string]{
		{Key: "category"},
		{Key: ""},
		{Key: "manual_notes"},
	}

	got := FindKeys(items)
	if len(got) != 2 || got[0] != "category" || got[1] != "manual_notes" {
		t.Fatalf("FindKeys() = %+v", got)
	}
}

func TestToActionItems(t *testing.T) {
	t.Parallel()

	items := []ReadinessItem[string, string]{
		{Key: "category", SuggestedAction: "确认类目"},
		{Key: "manual_notes", SuggestedAction: "处理备注"},
	}

	got := ToActionItems(items)
	if len(got) != 2 {
		t.Fatalf("ToActionItems() len = %d, want 2", len(got))
	}
	if got[0].Key != "category" || got[0].SuggestedAction != "确认类目" {
		t.Fatalf("ToActionItems()[0] = %+v", got[0])
	}
}
