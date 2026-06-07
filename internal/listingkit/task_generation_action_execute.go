package listingkit

import "context"

type taskGenerationActionExecutePhase struct {
	service *taskGenerationService
}

type taskGenerationActionPersistPhase struct {
	service *taskGenerationService
}

type taskGenerationActionFinalizePhase struct{}

type taskGenerationActionExecution struct {
	retryPage          *GenerationTaskPage
	queuePage          *GenerationQueuePage
	persistenceSession *GenerationReviewSession
}

func buildTaskGenerationActionExecutePhase(service *taskGenerationService) *taskGenerationActionExecutePhase {
	return &taskGenerationActionExecutePhase{service: service}
}

func buildTaskGenerationActionPersistPhase(service *taskGenerationService) *taskGenerationActionPersistPhase {
	return &taskGenerationActionPersistPhase{service: service}
}

func buildTaskGenerationActionFinalizePhase() *taskGenerationActionFinalizePhase {
	return &taskGenerationActionFinalizePhase{}
}

func (p *taskGenerationActionExecutePhase) run(ctx context.Context, taskID string, baseResult *ListingKitResult, target *AssetGenerationActionTarget) (*taskGenerationActionExecution, error) {
	handoff, err := buildTaskGenerationActionExecuteRequestHandoffPhase(p.service).run(ctx, taskID, target)
	if err != nil {
		return nil, err
	}

	return &taskGenerationActionExecution{
		retryPage:          handoff.retryPage,
		queuePage:          handoff.queuePage,
		persistenceSession: buildGenerationReviewSession(baseResult, handoff.persistenceQueue, target.QueueQuery),
	}, nil
}

func (p *taskGenerationActionPersistPhase) run(ctx context.Context, taskID string, target *AssetGenerationActionTarget, execution *taskGenerationActionExecution) error {
	if target == nil || !isPersistedGenerationReviewAction(target.ActionKey) || p.service.persistGenerationReviewDecision == nil {
		return nil
	}
	if _, err := p.service.persistGenerationReviewDecision(ctx, taskID, target.ActionKey, execution.persistenceSession, target); err != nil {
		return err
	}
	return nil
}

func (p *taskGenerationActionFinalizePhase) run(
	result *GenerationActionExecutionResult,
	projection *GenerationActionExecutionResult,
) *GenerationActionExecutionResult {
	if result == nil {
		result = &GenerationActionExecutionResult{}
	}
	if projection != nil {
		result.Overview = projection.Overview
		result.Queue = projection.Queue
		result.Retry = projection.Retry
		result.ReviewWorkflow = projection.ReviewWorkflow
		result.ReviewSession = projection.ReviewSession
		result.ReviewPatch = projection.ReviewPatch
		result.PlatformRenderPreviews = projection.PlatformRenderPreviews
		result.DeltaToken = projection.DeltaToken
	}
	return applyGenerationConditionalStateToActionResult(result)
}
