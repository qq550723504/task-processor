package shein

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildStatusOverviewBlockedState(t *testing.T) {
	t.Parallel()

	readiness := &SubmitStateInput{
		Status: "blocked",
		Summary: []string{
			"当前仍有关键字段未完成",
		},
		BlockingItems: []ActionItem{
			{Key: "category", SuggestedAction: "确认类目"},
		},
		WarningItems: []ActionItem{
			{Key: "manual_notes", SuggestedAction: "处理备注"},
		},
	}
	inspection := &sheinpub.Inspection{
		NeedsReview: true,
		Summary:     []string{"类目和规格仍需人工确认"},
		Sections: []sheinpub.InspectionSection{
			{Title: "类目解析", Status: "missing", ActionItems: []string{"确认类目"}},
			{Title: "规格", Status: "partial", ActionItems: []string{"确认规格"}},
		},
	}

	overview := BuildStatusOverview(inspection, readiness)
	if overview == nil {
		t.Fatal("expected status overview")
	}
	if overview.Status != "blocked" {
		t.Fatalf("status = %q", overview.Status)
	}
	if overview.PrimaryAction != "确认类目" || overview.PrimaryActionKey != "category" {
		t.Fatalf("primary action = %+v", overview)
	}
	if !overview.NeedsReview || len(overview.Highlights) == 0 || len(overview.NextActions) == 0 {
		t.Fatalf("overview = %+v", overview)
	}
}

func TestBuildWorkspaceOverviewUsesRepairSessionAsPrimaryEntry(t *testing.T) {
	t.Parallel()

	status := &StatusOverview{
		Status:           "blocked",
		Headline:         "SHEIN 资料包暂不能直接提交",
		Subheadline:      "当前仍有关键字段未完成",
		PrimaryAction:    "确认类目",
		PrimaryActionKey: "category",
		NeedsReview:      true,
		BlockingCount:    2,
		WarningCount:     1,
		Highlights:       []string{"类目骨架待处理"},
		NextActions:      []string{"确认类目"},
	}
	readiness := &SubmitStateInput{
		Status:        "blocked",
		Ready:         false,
		Summary:       []string{"当前仍有关键字段未完成"},
		BlockingItems: []ActionItem{{Key: "category", SuggestedAction: "确认类目"}},
		WarningItems:  []ActionItem{{Key: "manual_notes", SuggestedAction: "处理备注"}},
	}
	repair := &RepairStateInput{
		Status:             "needs_repair",
		TotalActions:       3,
		DirectApplyActions: 1,
		PrimaryPlanStatus:  "mixed",
		SessionStatus:      "guided_mixed",
		Summary:            []string{"已整理 3 个修复动作"},
		Session: &SessionInput{
			Status:        "guided_mixed",
			CurrentStepID: "step-2",
			NextStepID:    "step-3",
			ResumeMode:    "editor_required",
			RefreshBlocks: []string{"inspection", "submit_readiness"},
		},
	}

	overview := BuildWorkspaceOverview(status, readiness, repair)
	if overview == nil {
		t.Fatal("expected workspace overview")
	}
	if overview.PrimaryView != "repair_center" {
		t.Fatalf("primary view = %q", overview.PrimaryView)
	}
	if overview.ActiveSession == nil || overview.ActiveSession.CurrentStepID != "step-2" {
		t.Fatalf("active session = %+v", overview.ActiveSession)
	}
	if overview.RepairState == nil || overview.RepairState.TotalActions != 3 {
		t.Fatalf("repair state = %+v", overview.RepairState)
	}
}
