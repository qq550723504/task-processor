package workspace

import "testing"

type repairCenterTestHint struct {
	priority      string
	target        string
	editorSection string
	revisionPath  string
	description   string
	revision      *string
	validation    *repairCenterTestValidation
}

type repairCenterTestValidation struct {
	valid       bool
	changeCount int
}

func TestBuildRepairCenterFromReadinessAggregatesAndSortsActions(t *testing.T) {
	t.Parallel()

	reason := "category unresolved"
	revision := "apply category"
	readiness := &SubmitReadiness[string, repairCenterTestHint]{
		BlockingItems: []ReadinessItem[string, repairCenterTestHint]{
			{
				Key:             "category",
				Label:           "类目骨架",
				SuggestedAction: "确认类目",
				Reason:          &reason,
				RepairHints: []repairCenterTestHint{
					{
						priority:      "high",
						target:        "editor.category",
						editorSection: "category",
						revisionPath:  "shein.category_resolution",
						revision:      &revision,
						validation:    &repairCenterTestValidation{valid: true, changeCount: 2},
					},
				},
			},
		},
		WarningItems: []ReadinessItem[string, repairCenterTestHint]{
			{
				Key:             "manual_notes",
				Label:           "人工备注",
				SuggestedAction: "处理备注",
				RepairHints: []repairCenterTestHint{
					{
						priority:      "medium",
						target:        "editor.basics.review_notes",
						editorSection: "basics",
						revisionPath:  "shein.review_notes",
						validation:    &repairCenterTestValidation{valid: true},
					},
				},
			},
		},
	}
	checklist := &SubmitChecklist[string, repairCenterTestHint]{
		Required: []ChecklistGroupItem[string, repairCenterTestHint]{
			{
				Key:             "category",
				Label:           "类目骨架",
				Status:          "blocking",
				SuggestedAction: "确认类目",
				Reason:          &reason,
				RepairHints: []repairCenterTestHint{
					{
						priority:      "high",
						target:        "editor.category",
						editorSection: "category",
						revisionPath:  "shein.category_resolution",
						revision:      &revision,
						validation:    &repairCenterTestValidation{valid: true, changeCount: 2},
					},
				},
			},
		},
	}

	center := BuildRepairCenterFromReadiness(
		readiness,
		checklist,
		repairCenterTestHintAccessors(),
		RepairCenterFromReadinessOptions[string, string, string, string, repairCenterTestValidation]{
			CloneReason: func(value *string) *string {
				if value == nil {
					return nil
				}
				cloned := *value
				return &cloned
			},
			ValidationValid: func(validation *repairCenterTestValidation) bool {
				return validation != nil && validation.valid
			},
			ChangeCount: func(validation *repairCenterTestValidation) int {
				if validation == nil {
					return 0
				}
				return validation.changeCount
			},
			ReasonSummary: func(value *string) string {
				if value == nil {
					return ""
				}
				return *value
			},
			ActionInfo: func(action RepairCenterAction[string, string, string, string, repairCenterTestValidation]) RepairSessionActionInfo {
				info := RepairSessionActionInfo{
					ID:               action.ID,
					CanApplyDirectly: action.CanApplyDirectly,
				}
				if action.Validation != nil && action.Validation.valid {
					info.ValidationValid = true
					info.AffectedSections = []string{action.EditorSection}
				}
				return info
			},
		},
	)

	if center == nil {
		t.Fatal("expected repair center")
	}
	if center.Stats == nil || center.Stats.TotalActions != 2 {
		t.Fatalf("stats = %+v", center.Stats)
	}
	if center.Stats.BlockingActions != 1 || center.Stats.DirectApplyActions != 1 {
		t.Fatalf("stats = %+v", center.Stats)
	}
	if len(center.Actions) != 2 || center.Actions[0].Key != "category" || center.Actions[1].Key != "manual_notes" {
		t.Fatalf("actions = %+v", center.Actions)
	}
	if got := center.Actions[0].SourceGroups; len(got) != 2 || got[0] != "blocking" || got[1] != "required" {
		t.Fatalf("source groups = %+v", got)
	}
	if center.PrimaryPlan == nil || center.PrimaryPlan.Status != "mixed" {
		t.Fatalf("primary plan = %+v", center.PrimaryPlan)
	}
	if center.ApplyQueue == nil || center.ApplyQueue.Status != "partial_ready" || center.ApplyQueue.ReadyActions != 1 {
		t.Fatalf("apply queue = %+v", center.ApplyQueue)
	}
	if center.Session == nil || center.Session.Status != "guided_mixed" {
		t.Fatalf("session = %+v", center.Session)
	}
	if len(center.Sections) != 2 || center.Sections[0].Key != "category" || center.Sections[0].Label != "类目修复" {
		t.Fatalf("sections = %+v", center.Sections)
	}
}

func repairCenterTestHintAccessors() RepairHintAccessors[repairCenterTestHint, string, string, string, repairCenterTestValidation] {
	return RepairHintAccessors[repairCenterTestHint, string, string, string, repairCenterTestValidation]{
		Priority: func(hint repairCenterTestHint) string {
			return hint.priority
		},
		Target: func(hint repairCenterTestHint) string {
			return hint.target
		},
		EditorSection: func(hint repairCenterTestHint) string {
			return hint.editorSection
		},
		EditorFocus: func(repairCenterTestHint) []string {
			return nil
		},
		RevisionPath: func(hint repairCenterTestHint) string {
			return hint.revisionPath
		},
		Description: func(hint repairCenterTestHint) string {
			return hint.description
		},
		Patch: func(repairCenterTestHint) *string {
			return nil
		},
		Skeleton: func(repairCenterTestHint) *string {
			return nil
		},
		Revision: func(hint repairCenterTestHint) *string {
			return hint.revision
		},
		Validation: func(hint repairCenterTestHint) *repairCenterTestValidation {
			return hint.validation
		},
	}
}
