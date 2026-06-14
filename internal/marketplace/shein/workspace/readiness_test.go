package workspace

import "testing"

func TestBuildSubmitReadinessAndChecklist(t *testing.T) {
	readiness := BuildSubmitReadiness(
		[]ReadinessCheckSpec{
			{Key: "category", Label: "类目", OK: false, SuggestedAction: "确认类目"},
			{Key: "request_draft", Label: "草稿", OK: true},
		},
		func(spec ReadinessCheckSpec) Guidance[string, string] { return Guidance[string, string]{} },
		"blocked",
		"warnings",
		"ready",
	)

	if readiness == nil || readiness.Status != "blocked" || readiness.Ready {
		t.Fatalf("readiness = %#v", readiness)
	}
	checklist := BuildSubmitChecklist(readiness, SubmitChecklistGroupForKey)
	if checklist == nil || len(checklist.Required) != 1 || len(checklist.Recommended) != 1 {
		t.Fatalf("checklist = %#v", checklist)
	}
	actions := ToActionItems(readiness.BlockingItems)
	if len(actions) != 1 || actions[0].Key != "category" || actions[0].SuggestedAction != "确认类目" {
		t.Fatalf("actions = %#v", actions)
	}
}
