package listingkit

import listinggeneration "task-processor/internal/listingkit/generation"

type taskGenerationActionProjectionFinalizePhase struct{}

func buildTaskGenerationActionProjectionFinalizePhase() *taskGenerationActionProjectionFinalizePhase {
	return &taskGenerationActionProjectionFinalizePhase{}
}

func (p *taskGenerationActionProjectionFinalizePhase) run(
	input *taskGenerationActionProjectionInput,
	result *GenerationActionExecutionResult,
	reviewSession *GenerationReviewSession,
) *GenerationActionExecutionResult {
	if result == nil {
		result = &GenerationActionExecutionResult{}
	}

	result.ReviewSession = nil
	if reviewSession != nil {
		result.ReviewSession = reviewSession
	}

	actionKey := ""
	var target *AssetGenerationActionTarget
	var previousReviewSession *GenerationReviewSession
	if input != nil {
		actionKey = input.actionKey
		target = input.target
		previousReviewSession = input.previousReviewSession
	}

	result.ReviewWorkflow = buildGenerationReviewWorkflowResult(actionKey, target)
	applyGenerationReviewWorkflow(result.ReviewSession, result.ReviewWorkflow)
	result.ReviewPatch = buildGenerationReviewSessionPatch(previousReviewSession, result.ReviewSession)
	if result.ReviewPatch != nil {
		result.ReviewPatch.LastWorkflowResult = result.ReviewWorkflow
		result.DeltaToken = result.ReviewPatch.DeltaToken
	}
	if result.DeltaToken == "" {
		result.DeltaToken = buildGenerationReviewDeltaToken(result.ReviewSession)
	}
	if result.ResponseMode == "patch_only" {
		result.ReviewSession = nil
		result.PlatformRenderPreviews = nil
	}

	return result
}

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
