package listingkit

import "context"

func (s *taskGenerationService) executeLayerTemporalAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (bool, *GenerationActionExecutionResult, error) {
	actionKey := requestedAssetGenerationActionKey(req)
	switch actionKey {
	case assetGenerationActionRunStandardProductTemporal:
		result, err := buildTaskGenerationActionTemporalStandardPhase(s).run(ctx, taskID, req)
		return true, result, err
	case assetGenerationActionRunPlatformAdaptTemporal:
		result, err := buildTaskGenerationActionTemporalPlatformPhase(s).run(ctx, taskID, req)
		return true, result, err
	default:
		return false, nil, nil
	}
}

func (s *taskGenerationService) getCurrentAssetGenerationOverview(ctx context.Context, taskID string) (*AssetGenerationOverview, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationCurrentStateViewsPhase().overview(result), nil
}

func (s *taskGenerationService) getCurrentAssetGenerationQueue(ctx context.Context, taskID string) (*GenerationWorkQueue, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationCurrentStateViewsPhase().queue(result), nil
}

func (s *taskGenerationService) getCurrentActionRenderPreviews(ctx context.Context, taskID string, query *GenerationQueueQuery) ([]PlatformAssetRenderPreviews, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationCurrentStateViewsPhase().renderPreviews(result, query), nil
}

func (s *taskGenerationService) getCurrentListingKitResult(ctx context.Context, taskID string) (*ListingKitResult, error) {
	snapshot, err := buildTaskGenerationCurrentStateSnapshotPhase(s).run(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return snapshot.result, nil
}

func (s *taskGenerationService) dispatchGenerationNavigationPrimary(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationReviewNavigationDispatchResponse, error) {
	return buildTaskGenerationNavigationDispatchPrimaryPhase(s).run(ctx, taskID, target, responseMode)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {
	return buildTaskGenerationNavigationDispatchPlanPhase(s).run(ctx, taskID, target, responseMode)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanSequential(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	buildTaskGenerationNavigationDispatchStepExecutionPhase(s).runSequential(ctx, taskID, responseMode, plan, execution)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanParallel(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	buildTaskGenerationNavigationDispatchPlanParallelPhase(s).run(ctx, taskID, responseMode, plan, execution)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanStep(ctx context.Context, taskID string, step GenerationNavigationDispatchStep, responseMode string) *GenerationNavigationDispatchExecutionStep {
	return buildTaskGenerationNavigationDispatchStepExecutionPhase(s).run(ctx, taskID, step, responseMode)
}
