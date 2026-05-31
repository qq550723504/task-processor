package listingkit

import (
	"context"
	"time"
)

type taskGenerationActionEntryPhase struct {
	service *taskGenerationService
}

type taskGenerationActionEntryResult struct {
	queue                 *GenerationWorkQueue
	baseResult            *ListingKitResult
	target                *AssetGenerationActionTarget
	previousReviewSession *GenerationReviewSession
	result                *GenerationActionExecutionResult
}

func buildTaskGenerationActionEntryPhase(service *taskGenerationService) *taskGenerationActionEntryPhase {
	return &taskGenerationActionEntryPhase{service: service}
}

func (p *taskGenerationActionEntryPhase) run(
	ctx context.Context,
	taskID string,
	req *ExecuteGenerationActionRequest,
) (*taskGenerationActionEntryResult, error) {
	queue, err := p.service.getCurrentAssetGenerationQueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	baseResult, err := p.service.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	overview := buildAssetGenerationOverview(queue)
	target, source, err := resolveAssetGenerationActionTarget(overview, req)
	if err != nil {
		return nil, err
	}
	if target.ExpectedImpact == nil {
		target.ExpectedImpact = buildAssetGenerationActionImpact(queue, target.QueueQuery)
	}
	previousReviewSession := buildGenerationReviewSession(baseResult, queue, target.QueueQuery)
	result := &GenerationActionExecutionResult{
		ActionKey:       target.ActionKey,
		InteractionMode: target.InteractionMode,
		ResponseMode:    normalizeGenerationActionResponseMode(req.ResponseMode),
		ResolvedTarget:  target,
		Audit: &GenerationActionAudit{
			RequestedActionKey: requestedAssetGenerationActionKey(req),
			ResolvedActionKey:  target.ActionKey,
			ResolutionSource:   source,
			ExecutionPath:      target.InteractionMode,
			ExecutedAt:         time.Now().UTC(),
		},
	}

	return &taskGenerationActionEntryResult{
		queue:                 queue,
		baseResult:            baseResult,
		target:                target,
		previousReviewSession: previousReviewSession,
		result:                result,
	}, nil
}
