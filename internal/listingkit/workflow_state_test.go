package listingkit

import "testing"

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
