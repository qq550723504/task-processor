package workspace

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildStatusOverviewBlockedState(t *testing.T) {
	inspection := &sheinpub.Inspection{
		NeedsReview: true,
		Sections: []sheinpub.InspectionSection{
			{Title: "类目解析", Status: "partial", Actions: []sheinpub.InspectionAction{{Label: "确认类目"}}},
			{Title: "普通属性映射", Status: "resolved"},
			{Title: "销售属性选择", Status: "missing"},
		},
	}
	readiness := &SubmitStateInput{
		Status: "blocked",
		Summary: []string{
			"当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态",
		},
		BlockingItems: []ActionItem{
			{Key: "category", SuggestedAction: "确认类目"},
			{Key: "sale_attributes", SuggestedAction: "确认规格"},
		},
	}

	overview := BuildStatusOverview(inspection, readiness)
	if overview == nil {
		t.Fatal("expected overview")
	}
	if overview.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", overview.Status)
	}
	if overview.PrimaryAction != "确认类目" || overview.PrimaryActionKey != "category" {
		t.Fatalf("primary action = %+v", overview)
	}
	if overview.BlockingCount != 2 {
		t.Fatalf("blocking_count = %d, want 2", overview.BlockingCount)
	}
	if len(overview.Highlights) == 0 || len(overview.NextActions) == 0 {
		t.Fatalf("overview = %+v, want highlights and next actions", overview)
	}
}

func TestBuildWorkspaceOverviewUsesRepairSessionAsPrimaryEntry(t *testing.T) {
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
		Ready:         false,
		Status:        "blocked",
		Summary:       []string{"当前仍有关键字段未完成"},
		BlockingItems: []ActionItem{{Key: "category"}},
		WarningItems:  []ActionItem{{Key: "manual_notes"}},
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
	if overview.ActiveSession == nil || overview.ActiveSession.CurrentStepID != "step-2" || overview.ActiveSession.ResumeMode != "editor_required" {
		t.Fatalf("active session = %+v", overview.ActiveSession)
	}
	if overview.SubmitState == nil || !overview.NeedsReview {
		t.Fatalf("submit state = %+v", overview.SubmitState)
	}
	if overview.RepairState == nil || overview.RepairState.TotalActions != 3 || overview.RepairState.SessionStatus != "guided_mixed" {
		t.Fatalf("repair state = %+v", overview.RepairState)
	}
}
