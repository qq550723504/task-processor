package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *service) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {
	queue, err := s.getCurrentAssetGenerationQueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	baseResult, err := s.getCurrentListingKitResult(ctx, taskID)
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
	switch target.InteractionMode {
	case "retryable":
		retryPage, err := s.RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(target.RetryRequest))
		if err != nil {
			return nil, err
		}
		result.Retry = retryPage
	default:
		queuePage, err := s.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
		if err != nil {
			return nil, err
		}
		result.Queue = queuePage
	}
	if isPersistedGenerationReviewAction(target.ActionKey) {
		var persistenceSession *GenerationReviewSession
		switch target.InteractionMode {
		case "retryable":
			persistenceSession = buildGenerationReviewSession(baseResult, generationWorkQueueFromRetryPage(result.Retry), target.QueueQuery)
		default:
			persistenceSession = buildGenerationReviewSession(baseResult, generationWorkQueueFromPage(result.Queue), target.QueueQuery)
		}
		if _, err := s.persistGenerationReviewDecision(ctx, taskID, target.ActionKey, persistenceSession, target); err != nil {
			return nil, err
		}
	}
	result.Overview, err = s.getCurrentAssetGenerationOverview(ctx, taskID)
	if err != nil {
		return nil, err
	}
	result.PlatformRenderPreviews, err = s.getCurrentActionRenderPreviews(ctx, taskID, target.QueueQuery)
	if err != nil {
		return nil, err
	}
	if len(result.PlatformRenderPreviews) == 0 {
		result.PlatformRenderPreviews = buildActionPlatformRenderPreviews(baseResult, target.QueueQuery)
	}
	currentResult, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(currentResult.PlatformAssetRenderPreviews) == 0 && len(result.PlatformRenderPreviews) > 0 {
		currentResult.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), result.PlatformRenderPreviews...)
	}
	if len(currentResult.AssetRenderPreviews) == 0 && baseResult != nil {
		currentResult.AssetRenderPreviews = append([]AssetRenderPreview(nil), baseResult.AssetRenderPreviews...)
	}
	switch target.InteractionMode {
	case "retryable":
		result.ReviewSession = buildGenerationReviewSession(currentResult, generationWorkQueueFromRetryPage(result.Retry), target.QueueQuery)
	default:
		result.ReviewSession = buildGenerationReviewSession(currentResult, generationWorkQueueFromPage(result.Queue), target.QueueQuery)
	}
	result.ReviewWorkflow = buildGenerationReviewWorkflowResult(target.ActionKey, target)
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
	return applyGenerationConditionalStateToActionResult(result), nil
}

func (s *service) getCurrentAssetGenerationOverview(ctx context.Context, taskID string) (*AssetGenerationOverview, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return result.AssetGenerationOverview, nil
}

func (s *service) getCurrentAssetGenerationQueue(ctx context.Context, taskID string) (*GenerationWorkQueue, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return result.AssetGenerationQueue, nil
}

func (s *service) getCurrentActionRenderPreviews(ctx context.Context, taskID string, query *GenerationQueueQuery) ([]PlatformAssetRenderPreviews, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildActionPlatformRenderPreviews(result, query), nil
}

func (s *service) getCurrentListingKitResult(ctx context.Context, taskID string) (*ListingKitResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	reviews, err := s.listGenerationReviews(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	return withListingKitResultGenerationAndReview(task.Result, tasks, reviews), nil
}

func resolveAssetGenerationActionTarget(overview *AssetGenerationOverview, req *ExecuteGenerationActionRequest) (*AssetGenerationActionTarget, string, error) {
	actionKey := requestedAssetGenerationActionKey(req)
	if actionKey == "" {
		return nil, "", fmt.Errorf("%w: missing action key", ErrGenerationActionNotFound)
	}
	if !isAllowedAssetGenerationActionKey(actionKey) {
		return nil, "", fmt.Errorf("%w: %s", ErrGenerationActionNotFound, actionKey)
	}
	if overview != nil {
		for _, candidate := range collectAssetGenerationActionTargets(overview) {
			if candidate == nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(candidate.ActionKey), actionKey) {
				return cloneAssetGenerationActionTarget(candidate), "overview", nil
			}
		}
	}
	if req != nil && req.Target != nil && strings.EqualFold(strings.TrimSpace(req.Target.ActionKey), actionKey) {
		cloned := cloneAssetGenerationActionTarget(req.Target)
		if strings.TrimSpace(cloned.InteractionMode) == "" {
			cloned.InteractionMode = actionInteractionMode(cloned.ActionKey)
		}
		return cloned, "request_target", nil
	}
	return nil, "", fmt.Errorf("%w: %s", ErrGenerationActionNotFound, actionKey)
}

func collectAssetGenerationActionTargets(overview *AssetGenerationOverview) []*AssetGenerationActionTarget {
	if overview == nil {
		return nil
	}
	out := make([]*AssetGenerationActionTarget, 0, 1+len(overview.SecondaryActionTargets))
	if overview.PrimaryActionTarget != nil {
		out = append(out, overview.PrimaryActionTarget)
	}
	out = append(out, overview.SecondaryActionTargets...)
	return out
}

func cloneAssetGenerationActionTarget(target *AssetGenerationActionTarget) *AssetGenerationActionTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	cloned.Filters = cloneAssetGenerationFilters(target.Filters)
	cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	cloned.RetryRequest = cloneRetryGenerationTasksRequest(target.RetryRequest)
	cloned.ExpectedImpact = cloneAssetGenerationActionImpact(target.ExpectedImpact)
	cloned.NavigationTarget = cloneGenerationReviewNavigationTarget(target.NavigationTarget)
	return &cloned
}

func cloneAssetGenerationActionImpact(impact *AssetGenerationActionImpact) *AssetGenerationActionImpact {
	if impact == nil {
		return nil
	}
	cloned := *impact
	cloned.Platforms = append([]string(nil), impact.Platforms...)
	cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)
	cloned.States = append([]string(nil), impact.States...)
	return &cloned
}

func cloneGenerationQueueQuery(query *GenerationQueueQuery) *GenerationQueueQuery {
	if query == nil {
		return nil
	}
	cloned := *query
	return &cloned
}

func cloneRetryGenerationTasksRequest(req *RetryGenerationTasksRequest) *RetryGenerationTasksRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	cloned.TaskIDs = append([]string(nil), req.TaskIDs...)
	cloned.Slots = append([]string(nil), req.Slots...)
	return &cloned
}

func requestedAssetGenerationActionKey(req *ExecuteGenerationActionRequest) string {
	if req == nil {
		return ""
	}
	actionKey := strings.TrimSpace(req.ActionKey)
	if actionKey == "" && req.Target != nil {
		actionKey = strings.TrimSpace(req.Target.ActionKey)
	}
	return actionKey
}
