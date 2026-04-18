package listingkit

func buildGenerationReviewWorkflowResult(actionKey string, target *AssetGenerationActionTarget) *GenerationReviewWorkflowResult {
	if target == nil {
		return nil
	}
	query := target.QueueQuery
	result := &GenerationReviewWorkflowResult{
		ActionKey: actionKey,
		Status:    "applied",
	}
	if query != nil {
		result.Platform = query.Platform
		result.Slot = query.Slot
		result.Capability = query.PreviewCapability
	}
	switch actionKey {
	case assetGenerationActionApproveSectionReview:
		result.Message = "Section review marked as approved."
	case assetGenerationActionDeferSectionReview:
		result.Message = "Section review deferred for later follow-up."
	case assetGenerationActionRetrySectionGeneration:
		result.Message = "Section generation retried for the selected review capability."
	default:
		result.Message = "Review workflow executed."
	}
	return result
}

func applyGenerationReviewWorkflow(session *GenerationReviewSession, workflow *GenerationReviewWorkflowResult) {
	if session == nil || workflow == nil {
		return
	}
	session.LastWorkflowResult = workflow
	for i := range session.Sections {
		if workflow.Capability != "" && session.Sections[i].Capability != workflow.Capability {
			continue
		}
		switch workflow.ActionKey {
		case assetGenerationActionApproveSectionReview:
			session.Sections[i].WorkflowState = "approved"
			session.Sections[i].WorkflowMessage = workflow.Message
		case assetGenerationActionDeferSectionReview:
			session.Sections[i].WorkflowState = "deferred"
			session.Sections[i].WorkflowMessage = workflow.Message
		case assetGenerationActionRetrySectionGeneration:
			session.Sections[i].WorkflowState = "retrying"
			session.Sections[i].WorkflowMessage = workflow.Message
		default:
			session.Sections[i].WorkflowState = "updated"
			session.Sections[i].WorkflowMessage = workflow.Message
		}
	}
}
