package listingkit

import (
	"testing"

	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
)

func TestBuildSheinStatusOverviewBlockedState(t *testing.T) {
	t.Parallel()

	pkg := &SheinPackage{
		Inspection: &SheinInspection{
			NeedsReview: true,
			Sections: []SheinInspectionSection{
				{Title: "类目解析", Status: "partial", Actions: []SheinInspectionAction{{Label: "确认类目"}}},
				{Title: "普通属性映射", Status: "resolved"},
				{Title: "销售属性选择", Status: "missing"},
			},
		},
	}
	readiness := &SheinSubmitReadiness{
		Status: "blocked",
		Summary: []string{
			"当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态",
		},
		BlockingItems: []SheinReadinessItem{
			{Key: "category", Label: "类目骨架", SuggestedAction: "确认类目"},
			{Key: "sale_attributes", Label: "销售属性", SuggestedAction: "确认规格"},
		},
	}

	overview := sheinworkspace.BuildStatusOverview(pkg.Inspection, sheinworkspace.BuildSubmitStateInput(readiness))
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

func TestBuildSheinStatusOverviewReadyState(t *testing.T) {
	t.Parallel()

	pkg := &SheinPackage{
		Inspection: &SheinInspection{
			Sections: []SheinInspectionSection{
				{Title: "类目解析", Status: "resolved"},
				{Title: "普通属性映射", Status: "resolved"},
				{Title: "销售属性选择", Status: "resolved"},
			},
		},
	}
	readiness := &SheinSubmitReadiness{
		Status:  "ready",
		Ready:   true,
		Summary: []string{"SHEIN 资料包已具备提交前所需的关键骨架"},
	}

	overview := sheinworkspace.BuildStatusOverview(pkg.Inspection, sheinworkspace.BuildSubmitStateInput(readiness))
	if overview == nil {
		t.Fatal("expected overview")
	}
	if overview.Status != "ready" {
		t.Fatalf("status = %q, want ready", overview.Status)
	}
	if overview.NeedsReview {
		t.Fatalf("needs_review = true, want false; overview=%+v", overview)
	}
	if overview.PrimaryAction != "" {
		t.Fatalf("primary_action = %q, want empty", overview.PrimaryAction)
	}
}

func TestBuildSheinWorkspaceOverviewUsesRepairSessionAsPrimaryEntry(t *testing.T) {
	t.Parallel()

	status := &sheinworkspace.StatusOverview{
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
	readiness := &SheinSubmitReadiness{
		Ready:         false,
		Status:        "blocked",
		Summary:       []string{"当前仍有关键字段未完成"},
		BlockingItems: []SheinReadinessItem{{Key: "category"}},
		WarningItems:  []SheinReadinessItem{{Key: "manual_notes"}},
	}
	center := &SheinRepairCenter{
		Status: "needs_repair",
		Stats: &SheinRepairCenterStats{
			TotalActions:       3,
			DirectApplyActions: 1,
		},
		PrimaryPlan: &SheinRepairPlan{Status: "mixed"},
		Session: &SheinRepairSession{
			Status:        "guided_mixed",
			CurrentStepID: "step-2",
			NextStepID:    "step-3",
			RefreshBlocks: []string{"inspection", "submit_readiness"},
			ResumeState: &SheinRepairResumeState{
				ResumeMode: "editor_required",
			},
		},
		Summary: []string{"已整理 3 个修复动作"},
	}

	overview := sheinworkspace.BuildWorkspaceOverview(status, sheinworkspace.BuildSubmitStateInput(readiness), sheinworkspace.BuildRepairStateInput(center))
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
