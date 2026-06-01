package listingkit

import (
	"context"
	"strings"
	"testing"
)

func TestWorkflowRecorderKeepsStageHistoryAndAggregatesIssues(t *testing.T) {
	result := &ListingKitResult{Summary: &GenerationSummary{}}
	recorder := newWorkflowRecorder(result)

	first := recorder.Start("sds_design_sync", "sds-task-1")
	first.Complete()
	second := recorder.Start("sds_design_sync", "sds-task-2")
	second.Degrade("sds_render_failed", "SDS render failed", "blank template")
	recorder.AddIssue(WorkflowIssueSeverityReview, "shein_review", "shein_category_review", "Confirm SHEIN category", "")
	recorder.FinalizeSummary()

	if got, want := len(result.WorkflowStages), 2; got != want {
		t.Fatalf("workflow stages = %d, want %d: %+v", got, want, result.WorkflowStages)
	}
	if result.WorkflowStages[0].Kind != "sds_design_sync" || result.WorkflowStages[0].Status != WorkflowStageStatusCompleted {
		t.Fatalf("first stage = %+v, want completed sds_design_sync", result.WorkflowStages[0])
	}
	if result.WorkflowStages[1].Kind != "sds_design_sync" || result.WorkflowStages[1].Status != WorkflowStageStatusDegraded {
		t.Fatalf("second stage = %+v, want degraded sds_design_sync", result.WorkflowStages[1])
	}
	if got, want := len(result.WorkflowIssues), 2; got != want {
		t.Fatalf("workflow issues = %d, want %d: %+v", got, want, result.WorkflowIssues)
	}
	if result.Summary == nil {
		t.Fatal("summary = nil")
	}
	if result.Summary.IssueCount != 2 || result.Summary.WarningCount != 1 || result.Summary.ReviewCount != 1 || result.Summary.BlockingCount != 0 {
		t.Fatalf("summary = %+v, want issue/warning/review counts", result.Summary)
	}
	if !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs_review from review issue", result.Summary)
	}
	if got, want := reviewReasonsFromResult(result), []string{"Confirm SHEIN category"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("review reasons = %#v, want %#v", got, want)
	}
}

func TestWorkflowRecorderMarksBlockingFailure(t *testing.T) {
	result := &ListingKitResult{Summary: &GenerationSummary{}}
	recorder := newWorkflowRecorder(result)

	stage := recorder.Start("product_enrich", "product-task-1")
	stage.Fail("product_enrich_failed", "Product enrichment failed", "upstream timeout")
	recorder.FinalizeSummary()

	if got, want := len(result.WorkflowStages), 1; got != want {
		t.Fatalf("workflow stages = %d, want %d", got, want)
	}
	if result.WorkflowStages[0].Status != WorkflowStageStatusFailed {
		t.Fatalf("stage = %+v, want failed", result.WorkflowStages[0])
	}
	if got, want := len(result.WorkflowIssues), 1; got != want {
		t.Fatalf("workflow issues = %d, want %d", got, want)
	}
	if result.WorkflowIssues[0].Severity != WorkflowIssueSeverityBlocking {
		t.Fatalf("issue = %+v, want blocking", result.WorkflowIssues[0])
	}
	if result.Summary.BlockingCount != 1 || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want blocking count and needs_review", result.Summary)
	}
}

func TestWorkflowStageHandleReportsItsOwnRunningStatus(t *testing.T) {
	result := &ListingKitResult{Summary: &GenerationSummary{}}
	recorder := newWorkflowRecorder(result)

	platformStage := recorder.Start("asset_generation_platform", "")
	otherStage := recorder.Start("sds_design_sync", "")
	otherStage.Complete()

	if !platformStage.IsRunning() {
		t.Fatalf("platform stage = %+v, want running even when it is not the latest stage", result.WorkflowStages[0])
	}
	platformStage.Complete()
	if platformStage.IsRunning() {
		t.Fatalf("platform stage = %+v, want not running after completion", result.WorkflowStages[0])
	}
}

func TestRefreshSheinTaskResultStateKeepsCoverageWarningButSkipsReviewIssue(t *testing.T) {
	t.Parallel()

	coverageWarning := "coverage guard warning"
	result := &ListingKitResult{
		Shein: &SheinPackage{
			CategoryID:  10489,
			ReviewNotes: []string{coverageWarning},
			CategoryResolution: &SheinCategoryResolution{
				Status:     "resolved",
				CategoryID: 10489,
			},
			AttributeResolution: &SheinAttributeResolution{
				Status:        "resolved",
				ResolvedCount: 3,
			},
			SaleAttributeResolution: &SheinSaleAttributeResolution{
				Status:             "resolved",
				PrimaryAttributeID: 11,
			},
			Metadata: map[string]string{
				sheinVariantImageCoverageStatusKey:  "blocked",
				sheinVariantImageCoverageMessageKey: coverageWarning,
			},
		},
		Summary: &GenerationSummary{
			NeedsReview: true,
			Warnings:    []string{coverageWarning},
		},
		ReviewReasons: []string{coverageWarning},
	}

	svc := &service{}
	svc.refreshSheinTaskResultState(context.Background(), &Task{Result: result}, result)

	if result.Summary == nil || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs review retained for coverage-only refresh", result.Summary)
	}
	if !strings.Contains(strings.Join(result.Summary.Warnings, "\n"), coverageWarning) {
		t.Fatalf("summary warnings = %#v, want coverage warning retained", result.Summary.Warnings)
	}
	if !strings.Contains(strings.Join(result.ReviewReasons, "\n"), coverageWarning) {
		t.Fatalf("review reasons = %#v, want coverage warning retained", result.ReviewReasons)
	}
	if !strings.Contains(strings.Join(result.Shein.ReviewNotes, "\n"), coverageWarning) {
		t.Fatalf("shein review notes = %#v, want coverage warning retained", result.Shein.ReviewNotes)
	}
	for _, issue := range result.WorkflowIssues {
		if issue.Stage == "shein_review" && issue.Severity == WorkflowIssueSeverityReview && issue.Message == coverageWarning {
			t.Fatalf("workflow issues = %+v, coverage guard warning should not become shein_review issue after refresh", result.WorkflowIssues)
		}
	}
}
