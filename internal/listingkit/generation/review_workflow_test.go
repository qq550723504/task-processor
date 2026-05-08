package generation

import "testing"

func TestReviewDecisionRules(t *testing.T) {
	t.Parallel()

	if !IsPersistedReviewAction(" approve_section_review ") {
		t.Fatalf("IsPersistedReviewAction() should accept approve action with spaces")
	}
	if IsPersistedReviewAction(ActionRetrySectionGeneration) {
		t.Fatalf("IsPersistedReviewAction() should reject retry action")
	}
	if got := ReviewDecisionFromAction(ActionApproveSectionReview); got != ReviewDecisionApprove {
		t.Fatalf("ReviewDecisionFromAction() = %q, want %q", got, ReviewDecisionApprove)
	}
	if got := ReviewStatusFromDecision(ReviewDecisionDefer); got != "deferred" {
		t.Fatalf("ReviewStatusFromDecision() = %q, want deferred", got)
	}
	if got := ReviewStatusFromDecision(""); got != "pending" {
		t.Fatalf("ReviewStatusFromDecision(empty) = %q, want pending", got)
	}
}

func TestBuildReviewWorkflowResult(t *testing.T) {
	t.Parallel()

	got := BuildReviewWorkflowResult(ActionRetrySectionGeneration)
	if got.ActionKey != ActionRetrySectionGeneration || got.Status != "applied" {
		t.Fatalf("BuildReviewWorkflowResult() = %+v, want action with applied status", got)
	}
	if got.Message != "Section generation retried for the selected review capability." {
		t.Fatalf("BuildReviewWorkflowResult() message = %q", got.Message)
	}
	if state := ReviewWorkflowState(ActionRetrySectionGeneration); state != "retrying" {
		t.Fatalf("ReviewWorkflowState() = %q, want retrying", state)
	}
}
