package listingkit

import "testing"

func TestBuildSheinWorkspaceOverviewUsesRepairSessionAsPrimaryEntry(t *testing.T) {
	t.Parallel()

	status := &SheinStatusOverview{
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

	overview := buildSheinWorkspaceOverview(status, readiness, center)
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
