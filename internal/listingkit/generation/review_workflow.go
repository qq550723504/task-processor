package generation

import "strings"

const (
	ReviewDecisionApprove = "approve"
	ReviewDecisionDefer   = "defer"
)

type ReviewWorkflowResult struct {
	ActionKey string
	Status    string
	Message   string
}

func IsPersistedReviewAction(actionKey string) bool {
	switch strings.TrimSpace(actionKey) {
	case ActionApproveSectionReview, ActionDeferSectionReview:
		return true
	default:
		return false
	}
}

func ReviewDecisionFromAction(actionKey string) string {
	switch strings.TrimSpace(actionKey) {
	case ActionApproveSectionReview:
		return ReviewDecisionApprove
	case ActionDeferSectionReview:
		return ReviewDecisionDefer
	default:
		return ""
	}
}

func ReviewStatusFromDecision(decision string) string {
	switch decision {
	case ReviewDecisionApprove:
		return "approved"
	case ReviewDecisionDefer:
		return "deferred"
	default:
		return "pending"
	}
}

func BuildReviewWorkflowResult(actionKey string) ReviewWorkflowResult {
	result := ReviewWorkflowResult{
		ActionKey: actionKey,
		Status:    "applied",
	}
	switch actionKey {
	case ActionApproveSectionReview:
		result.Message = "Section review marked as approved."
	case ActionDeferSectionReview:
		result.Message = "Section review deferred for later follow-up."
	case ActionRetrySectionGeneration:
		result.Message = "Section generation retried for the selected review capability."
	default:
		result.Message = "Review workflow executed."
	}
	return result
}

func ReviewWorkflowState(actionKey string) string {
	switch actionKey {
	case ActionApproveSectionReview:
		return "approved"
	case ActionDeferSectionReview:
		return "deferred"
	case ActionRetrySectionGeneration:
		return "retrying"
	default:
		return "updated"
	}
}
