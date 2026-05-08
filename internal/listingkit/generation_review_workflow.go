package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

func buildGenerationReviewWorkflowResult(actionKey string, target *AssetGenerationActionTarget) *GenerationReviewWorkflowResult {
	if target == nil {
		return nil
	}
	workflow := listinggeneration.BuildReviewWorkflowResult(actionKey)
	query := target.QueueQuery
	result := &GenerationReviewWorkflowResult{
		ActionKey: workflow.ActionKey,
		Status:    workflow.Status,
		Message:   workflow.Message,
	}
	if query != nil {
		result.Platform = query.Platform
		result.Slot = query.Slot
		result.Capability = query.PreviewCapability
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
		session.Sections[i].WorkflowState = listinggeneration.ReviewWorkflowState(workflow.ActionKey)
		session.Sections[i].WorkflowMessage = workflow.Message
	}
}
