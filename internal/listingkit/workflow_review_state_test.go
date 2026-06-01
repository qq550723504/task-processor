package listingkit

import (
	"strings"
	"testing"
)

func TestApplySheinInspectionReviewToSummaryMarksResultNeedsReview(t *testing.T) {
	result := &ListingKitResult{
		Summary: &GenerationSummary{},
		Shein: &SheinPackage{
			Inspection: &SheinInspection{
				NeedsReview: true,
				Summary:     []string{"建议复核 SHEIN 类目"},
			},
		},
	}

	applySheinInspectionReviewToSummary(result)

	if result.Summary == nil || !result.Summary.NeedsReview {
		t.Fatalf("summary = %+v, want needs_review", result.Summary)
	}
	if got := strings.Join(result.Summary.Warnings, "\n"); !strings.Contains(got, "建议复核 SHEIN 类目") {
		t.Fatalf("summary warnings = %#v, want SHEIN inspection summary", result.Summary.Warnings)
	}
	if got := strings.Join(result.ReviewReasons, "\n"); !strings.Contains(got, "建议复核 SHEIN 类目") {
		t.Fatalf("review reasons = %#v, want SHEIN inspection summary", result.ReviewReasons)
	}
}

func TestAddSheinReviewWorkflowIssuesIgnoresOtherStageReasons(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		Summary: &GenerationSummary{
			NeedsReview: true,
		},
		WorkflowIssues: []WorkflowIssue{
			{
				Code:     "asset_generation_review",
				Stage:    "asset_generation_platform",
				Severity: WorkflowIssueSeverityReview,
				Message:  "other stage review",
			},
			{
				Code:     "asset_generation_blocking",
				Stage:    "asset_generation_platform",
				Severity: WorkflowIssueSeverityBlocking,
				Message:  "other stage blocking",
			},
		},
		Shein: &SheinPackage{
			Inspection: &SheinInspection{
				NeedsReview: true,
				Summary:     []string{"inspection review"},
			},
		},
	}

	addSheinReviewWorkflowIssues(result)

	for _, issue := range result.WorkflowIssues {
		if issue.Stage == "shein_review" && issue.Message == "other stage review" {
			t.Fatalf("workflow issues = %+v, should not rewrite other stage review into shein_review", result.WorkflowIssues)
		}
		if issue.Stage == "shein_review" && issue.Message == "other stage blocking" {
			t.Fatalf("workflow issues = %+v, should not rewrite other stage blocking into shein_review", result.WorkflowIssues)
		}
	}

	foundInspectionIssue := false
	for _, issue := range result.WorkflowIssues {
		if issue.Stage == "shein_review" && issue.Severity == WorkflowIssueSeverityReview && issue.Code == "shein_review_required" && issue.Message == "inspection review" {
			foundInspectionIssue = true
		}
	}
	if !foundInspectionIssue {
		t.Fatalf("workflow issues = %+v, want shein review issue for inspection review", result.WorkflowIssues)
	}
}
