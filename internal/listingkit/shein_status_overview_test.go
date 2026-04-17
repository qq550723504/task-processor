package listingkit

import "testing"

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

	overview := buildSheinStatusOverview(pkg, readiness)
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

	overview := buildSheinStatusOverview(pkg, readiness)
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
