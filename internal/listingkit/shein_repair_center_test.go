package listingkit

import "testing"

func TestBuildSheinRepairCenterAggregatesAndSortsActions(t *testing.T) {
	t.Parallel()

	readiness := &SheinSubmitReadiness{
		BlockingItems: []SheinReadinessItem{
			{
				Key:             "category",
				Label:           "类目骨架",
				SuggestedAction: "确认类目",
				Reason: &SheinReadinessReason{
					Code: "category_unresolved",
				},
				RepairHints: []SheinRepairHint{
					{
						Action:        "确认类目",
						Priority:      "high",
						Target:        "editor.category",
						EditorSection: "category",
						RevisionPath:  "shein.category_resolution",
						Revision: &ApplyRevisionRequest{
							Platform: "shein",
							Shein: &SheinRevisionInput{
								CategoryResolution: &SheinCategoryResolutionPatch{},
							},
						},
						Validation: &SheinRepairValidationPreview{
							Valid: true,
						},
					},
				},
			},
		},
		WarningItems: []SheinReadinessItem{
			{
				Key:             "manual_notes",
				Label:           "人工备注",
				SuggestedAction: "处理备注",
				RepairHints: []SheinRepairHint{
					{
						Action:        "处理备注",
						Priority:      "medium",
						Target:        "editor.basics.review_notes",
						EditorSection: "basics",
						RevisionPath:  "shein.review_notes",
						Validation: &SheinRepairValidationPreview{
							Valid: true,
						},
					},
				},
			},
		},
	}
	checklist := &SheinSubmitChecklist{
		Required: []SheinChecklistGroupItem{
			{
				Key:             "category",
				Label:           "类目骨架",
				Status:          "blocking",
				SuggestedAction: "确认类目",
				RepairHints: []SheinRepairHint{
					{
						Action:        "确认类目",
						Priority:      "high",
						Target:        "editor.category",
						EditorSection: "category",
						RevisionPath:  "shein.category_resolution",
						Revision: &ApplyRevisionRequest{
							Platform: "shein",
						},
						Validation: &SheinRepairValidationPreview{
							Valid: true,
						},
					},
				},
			},
		},
	}

	center := buildSheinRepairCenter(readiness, checklist)
	if center == nil {
		t.Fatal("expected repair center")
	}
	if center.Stats == nil || center.Stats.TotalActions != 2 {
		t.Fatalf("stats = %+v", center.Stats)
	}
	if center.Stats.BlockingActions != 1 || center.Stats.DirectApplyActions != 1 {
		t.Fatalf("stats = %+v", center.Stats)
	}
	if center.PrimaryAction == nil || center.PrimaryAction.Key != "category" {
		t.Fatalf("primary action = %+v", center.PrimaryAction)
	}
	if center.PrimaryPlan == nil || center.PrimaryPlan.TotalSteps != 2 || center.PrimaryPlan.PrimaryStepID == "" {
		t.Fatalf("primary plan = %+v", center.PrimaryPlan)
	}
	if center.PrimaryPlan.Status != "mixed" {
		t.Fatalf("primary plan status = %q", center.PrimaryPlan.Status)
	}
	if center.ApplyQueue == nil || center.ApplyQueue.TotalActions != 2 || center.ApplyQueue.ReadyActions != 1 {
		t.Fatalf("apply queue = %+v", center.ApplyQueue)
	}
	if center.ApplyQueue.Status != "partial_ready" || len(center.ApplyQueue.Items) != 1 || center.ApplyQueue.Items[0].ActionID != center.Actions[0].ID {
		t.Fatalf("apply queue = %+v", center.ApplyQueue)
	}
	if center.Session == nil || center.Session.CurrentStepID == "" || len(center.Session.Runbook) != 2 {
		t.Fatalf("session = %+v", center.Session)
	}
	if center.Session.Status != "guided_mixed" {
		t.Fatalf("session status = %q", center.Session.Status)
	}
	if center.Session.ResumeState == nil || center.Session.ResumeState.ResumeStepID != center.Session.Runbook[1].ID {
		t.Fatalf("resume state = %+v", center.Session.ResumeState)
	}
	if center.Session.CompletionSnapshot == nil || center.Session.CompletionSnapshot.TotalSteps != 2 || center.Session.CompletionSnapshot.CompletedSteps != 1 {
		t.Fatalf("completion snapshot = %+v", center.Session.CompletionSnapshot)
	}
	if len(center.Session.SkippedSteps) != 1 || center.Session.SkippedSteps[0] != center.Session.Runbook[1].ID {
		t.Fatalf("skipped steps = %+v", center.Session.SkippedSteps)
	}
	if center.Session.Runbook[0].ExecutionMode != "direct_apply" || center.Session.Runbook[0].NextStepID == "" {
		t.Fatalf("runbook first step = %+v", center.Session.Runbook[0])
	}
	if center.Session.Runbook[1].ExecutionMode != "editor_required" || center.Session.Runbook[1].AutoAdvance {
		t.Fatalf("runbook second step = %+v", center.Session.Runbook[1])
	}
	if len(center.Actions) != 2 || center.Actions[0].Key != "category" || center.Actions[1].Key != "manual_notes" {
		t.Fatalf("actions = %+v", center.Actions)
	}
	if len(center.Actions[0].SourceGroups) < 2 {
		t.Fatalf("source groups = %+v", center.Actions[0].SourceGroups)
	}
	if len(center.Sections) != 2 || center.Sections[0].Key != "category" {
		t.Fatalf("sections = %+v", center.Sections)
	}
	if center.Status != "needs_repair" {
		t.Fatalf("status = %q", center.Status)
	}
}
