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

func TestBuildSubmitReadinessCarriesBlockerTaxonomy(t *testing.T) {
	taxonomy := ReadinessTaxonomy{
		BlockerKey:          "missing_category",
		Severity:            "blocker",
		Domain:              "category",
		RepairTarget:        "category_review",
		RepairRoute:         "workspace.category",
		Recoverable:         true,
		RequiresEngineering: false,
	}
	readiness := BuildSubmitReadiness(
		[]ReadinessCheckSpec{
			{
				Key:      "category",
				Label:    "类目",
				OK:       false,
				Taxonomy: taxonomy,
			},
		},
		func(spec ReadinessCheckSpec) Guidance[string, string] { return Guidance[string, string]{} },
		"blocked",
		"warnings",
		"ready",
	)

	if readiness == nil || len(readiness.BlockingItems) != 1 || len(readiness.Checks) != 1 {
		t.Fatalf("readiness = %#v, want one blocking item and one check", readiness)
	}
	if readiness.BlockingItems[0].Taxonomy != taxonomy {
		t.Fatalf("blocking taxonomy = %#v, want %#v", readiness.BlockingItems[0].Taxonomy, taxonomy)
	}
	if readiness.Checks[0].Taxonomy != taxonomy {
		t.Fatalf("check taxonomy = %#v, want %#v", readiness.Checks[0].Taxonomy, taxonomy)
	}
	checklist := BuildSubmitChecklist(readiness, SubmitChecklistGroupForKey)
	if checklist == nil || len(checklist.Required) != 1 {
		t.Fatalf("checklist = %#v, want required item", checklist)
	}
	if checklist.Required[0].Taxonomy != taxonomy {
		t.Fatalf("checklist taxonomy = %#v, want %#v", checklist.Required[0].Taxonomy, taxonomy)
	}
}
