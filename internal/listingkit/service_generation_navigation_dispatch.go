package listingkit

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

func (s *service) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {
	if req == nil || req.Target == nil {
		return nil, fmt.Errorf("%w: missing navigation target", ErrGenerationActionNotFound)
	}
	target := cloneGenerationReviewNavigationTarget(req.Target)
	ApplyGenerationConditionalBaselineToNavigationTarget(target, "")

	responseMode := normalizeGenerationActionResponseMode(req.ResponseMode)
	planMode := normalizeGenerationNavigationDispatchPlanMode(req.PlanMode)
	response, err := s.dispatchGenerationNavigationPrimary(ctx, taskID, target, responseMode)
	if err != nil {
		return nil, err
	}
	response.PlanMode = planMode
	if planMode == "execute_plan" {
		executedPlan, err := s.executeGenerationNavigationDispatchPlan(ctx, taskID, target, responseMode)
		if err != nil {
			return nil, err
		}
		applyExecutedPlanToDispatchResponse(response, executedPlan)
	}
	return finalizeGenerationReviewNavigationDispatchResponse(response), nil
}

func (s *service) dispatchGenerationNavigationPrimary(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationReviewNavigationDispatchResponse, error) {
	switch normalizeGenerationReviewDispatchKind(target) {
	case "action":
		actionReq := &ExecuteGenerationActionRequest{
			ResponseMode: responseMode,
			Target:       cloneAssetGenerationActionTarget(target.ActionTarget),
		}
		if actionReq.Target == nil {
			return nil, fmt.Errorf("%w: missing action target", ErrGenerationActionNotFound)
		}
		actionReq.ActionKey = actionReq.Target.ActionKey
		action, err := s.ExecuteTaskGenerationAction(ctx, taskID, actionReq)
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:       taskID,
			DispatchKind: "action",
			ResponseMode: responseMode,
			DeltaToken:   action.DeltaToken,
			Action:       action,
		}, nil
	case "preview":
		preview, err := s.GetTaskGenerationReviewPreview(ctx, taskID, cloneGenerationQueueQuery(target.PreviewQuery))
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:        taskID,
			DispatchKind:  "preview",
			ResponseMode:  responseMode,
			DeltaToken:    preview.DeltaToken,
			ReviewPreview: preview,
		}, nil
	case "queue":
		queue, err := s.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:       taskID,
			DispatchKind: "queue",
			ResponseMode: responseMode,
			DeltaToken:   queue.DeltaToken,
			Queue:        queue,
		}, nil
	default:
		sessionQuery := cloneGenerationQueueQuery(target.SessionQuery)
		if sessionQuery == nil {
			sessionQuery = cloneGenerationQueueQuery(target.QueueQuery)
		}
		if sessionQuery == nil {
			sessionQuery = cloneGenerationQueueQuery(target.PreviewQuery)
		}
		if sessionQuery != nil && strings.TrimSpace(responseMode) != "" {
			sessionQuery.ResponseMode = responseMode
		}
		session, err := s.GetTaskGenerationReviewSession(ctx, taskID, sessionQuery)
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:        taskID,
			DispatchKind:  "session",
			ResponseMode:  responseMode,
			DeltaToken:    session.DeltaToken,
			ReviewSession: session,
		}, nil
	}
}

func (s *service) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {
	if target == nil || target.Descriptor == nil || target.Descriptor.DispatchPlan == nil {
		return nil, nil
	}
	plan := cloneGenerationNavigationDispatchPlan(target.Descriptor.DispatchPlan)
	if plan == nil {
		return nil, nil
	}
	execution := &GenerationNavigationDispatchExecution{
		Strategy: plan.Strategy,
		Steps:    make([]GenerationNavigationDispatchExecutionStep, 0, len(plan.Steps)),
	}
	if generationNavigationDispatchPlanRunsInParallel(plan) {
		s.executeGenerationNavigationDispatchPlanParallel(ctx, taskID, responseMode, plan, execution)
		applyGenerationNavigationDispatchExecutionRules(plan, execution)
		return execution, nil
	}
	s.executeGenerationNavigationDispatchPlanSequential(ctx, taskID, responseMode, plan, execution)
	applyGenerationNavigationDispatchExecutionRules(plan, execution)
	return execution, nil
}

func (s *service) executeGenerationNavigationDispatchPlanSequential(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	for index, step := range plan.Steps {
		stepResult := s.executeGenerationNavigationDispatchPlanStep(ctx, taskID, step, responseMode)
		execution.Steps = append(execution.Steps, *stepResult)
		applyGenerationNavigationDispatchExecutionStats(execution, stepResult)
		if stepResult.Status == "failed" && plan.StopOnError {
			execution.StopReason = "error"
		}
		if execution.StopReason == "" && shouldStopGenerationNavigationDispatchPlan(plan, stepResult) {
			execution.StopReason = generationNavigationDispatchPlanStopReason(plan, stepResult)
		}
		if execution.StopReason != "" {
			for remaining := index + 1; remaining < len(plan.Steps); remaining++ {
				next := plan.Steps[remaining]
				skipped := generationNavigationDispatchPlanSkippedStep(next, execution.StopReason)
				execution.Steps = append(execution.Steps, skipped)
				applyGenerationNavigationDispatchExecutionStats(execution, &skipped)
			}
			break
		}
	}
}

func (s *service) executeGenerationNavigationDispatchPlanParallel(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	type dedupeEntry struct {
		step   GenerationNavigationDispatchStep
		result *GenerationNavigationDispatchExecutionStep
	}
	entries := make([]dedupeEntry, 0, len(plan.Steps))
	indexByKey := make(map[string]int, len(plan.Steps))
	for _, step := range plan.Steps {
		key := generationNavigationDispatchStepDeduplicationKey(step, responseMode)
		if existing, ok := indexByKey[key]; ok {
			deduped := generationNavigationDispatchPlanDeduplicatedStep(step, key, existing)
			entries = append(entries, dedupeEntry{step: step, result: &deduped})
			continue
		}
		result := generationNavigationDispatchExecutionPendingStep(step, key, responseMode)
		indexByKey[key] = len(entries)
		entries = append(entries, dedupeEntry{step: step, result: result})
	}
	maxParallelism := plan.MaxParallelism
	if maxParallelism <= 0 {
		maxParallelism = 1
	}
	sem := make(chan struct{}, maxParallelism)
	var wg sync.WaitGroup
	for index := range entries {
		if entries[index].result.Status == "deduplicated" {
			continue
		}
		wg.Add(1)
		go func(entry *dedupeEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			entry.result = s.executeGenerationNavigationDispatchPlanStep(ctx, taskID, entry.step, responseMode)
			entry.result.DeduplicationKey = generationNavigationDispatchStepDeduplicationKey(entry.step, responseMode)
		}(&entries[index])
	}
	wg.Wait()
	for _, entry := range entries {
		stepResult := entry.result
		if stepResult == nil {
			continue
		}
		if stepResult.Status == "deduplicated" {
			if source := entry.result.DeduplicatedFrom; source >= 0 && source < len(entries) && entries[source].result != nil {
				stepResult.DeltaToken = entries[source].result.DeltaToken
				stepResult.NotModified = entries[source].result.NotModified
				stepResult.NoChanges = entries[source].result.NoChanges
			}
		}
		execution.Steps = append(execution.Steps, *stepResult)
		applyGenerationNavigationDispatchExecutionStats(execution, stepResult)
	}
}

func (s *service) executeGenerationNavigationDispatchPlanStep(ctx context.Context, taskID string, step GenerationNavigationDispatchStep, responseMode string) *GenerationNavigationDispatchExecutionStep {
	result := &GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       firstNonEmpty(step.ResponseMode, responseMode),
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		DeduplicationKey:   generationNavigationDispatchStepDeduplicationKey(step, responseMode),
		Executed:           true,
		Status:             "completed",
	}
	query := cloneGenerationQueueQuery(step.Query)
	if query != nil && strings.TrimSpace(result.ResponseMode) != "" {
		query.ResponseMode = result.ResponseMode
	}
	switch strings.ToLower(strings.TrimSpace(step.Kind)) {
	case "queue":
		queue, err := s.GetTaskGenerationQueue(ctx, taskID, query)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.ErrorKind = classifyGenerationNavigationDispatchStepError(err)
			return result
		}
		result.Queue = queue
		if queue != nil {
			result.DeltaToken = queue.DeltaToken
			result.NotModified = queue.NotModified
			result.NoChanges = queue.NotModified
			if queue.NotModified {
				result.Status = "not_modified"
			}
		}
	case "preview":
		preview, err := s.GetTaskGenerationReviewPreview(ctx, taskID, query)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.ErrorKind = classifyGenerationNavigationDispatchStepError(err)
			return result
		}
		result.ReviewPreview = preview
		if preview != nil {
			result.DeltaToken = preview.DeltaToken
			result.NotModified = preview.NotModified
			result.NoChanges = preview.NotModified
			if preview.NotModified {
				result.Status = "not_modified"
			}
		}
	default:
		session, err := s.GetTaskGenerationReviewSession(ctx, taskID, query)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.ErrorKind = classifyGenerationNavigationDispatchStepError(err)
			return result
		}
		result.ReviewSession = session
		if session != nil {
			result.DeltaToken = session.DeltaToken
			result.NotModified = session.NotModified
			result.NoChanges = session.NotModified
			if session.NotModified {
				result.Status = "not_modified"
			}
		}
	}
	return result
}

func shouldStopGenerationNavigationDispatchPlan(plan *GenerationNavigationDispatchPlan, step *GenerationNavigationDispatchExecutionStep) bool {
	if plan == nil || step == nil {
		return false
	}
	if plan.StopOnNotModified && step.NotModified {
		return true
	}
	if plan.StopOnFirstSuccess && step.Executed && !step.NotModified {
		return true
	}
	return false
}

func generationNavigationDispatchPlanStopReason(plan *GenerationNavigationDispatchPlan, step *GenerationNavigationDispatchExecutionStep) string {
	if plan == nil || step == nil {
		return ""
	}
	if plan.StopOnNotModified && step.NotModified {
		return "not_modified"
	}
	if plan.StopOnFirstSuccess && step.Executed && !step.NotModified {
		return "first_success"
	}
	return ""
}

func generationNavigationDispatchPlanRunsInParallel(plan *GenerationNavigationDispatchPlan) bool {
	if plan == nil {
		return false
	}
	return !plan.StopOnError && !plan.StopOnFirstSuccess && !plan.StopOnNotModified && len(plan.Steps) > 1
}

func generationNavigationDispatchStepDeduplicationKey(step GenerationNavigationDispatchStep, responseMode string) string {
	query := cloneGenerationQueueQuery(step.Query)
	if query != nil && strings.TrimSpace(query.ResponseMode) == "" {
		query.ResponseMode = firstNonEmpty(step.ResponseMode, responseMode)
	}
	return hashRenderRevision(
		strings.ToLower(strings.TrimSpace(step.Kind)),
		firstNonEmpty(step.ResponseMode, responseMode),
		firstNonEmpty(step.CachePreference, ""),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.Platform }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.Slot }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.PreviewCapability }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.AssetID }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.AssetRevision }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.PreviewRevision }), "")),
		strings.TrimSpace(firstNonEmpty(queryValue(query, func(q *GenerationQueueQuery) string { return q.TaskRevision }), "")),
	)
}

func generationNavigationDispatchExecutionPendingStep(step GenerationNavigationDispatchStep, dedupeKey string, responseMode string) *GenerationNavigationDispatchExecutionStep {
	return &GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       firstNonEmpty(step.ResponseMode, responseMode),
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		DeduplicationKey:   dedupeKey,
		Status:             "pending",
	}
}

func generationNavigationDispatchPlanDeduplicatedStep(step GenerationNavigationDispatchStep, dedupeKey string, sourceIndex int) GenerationNavigationDispatchExecutionStep {
	return GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       step.ResponseMode,
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		Status:             "deduplicated",
		DeduplicationKey:   dedupeKey,
		DeduplicatedFrom:   sourceIndex,
		Skipped:            true,
	}
}

func generationNavigationDispatchPlanSkippedStep(step GenerationNavigationDispatchStep, stopReason string) GenerationNavigationDispatchExecutionStep {
	return GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       step.ResponseMode,
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		Status:             "skipped",
		Error:              stopReason,
		Skipped:            true,
	}
}

func applyGenerationNavigationDispatchExecutionStats(execution *GenerationNavigationDispatchExecution, step *GenerationNavigationDispatchExecutionStep) {
	if execution == nil || step == nil {
		return
	}
	switch step.Status {
	case "failed":
		execution.FailedSteps++
		execution.Partial = true
	case "deduplicated":
		execution.DedupedSteps++
	case "skipped":
		execution.Partial = true
	default:
		if step.Executed {
			execution.CompletedSteps++
		}
	}
}

func queryValue(query *GenerationQueueQuery, selector func(*GenerationQueueQuery) string) string {
	if query == nil || selector == nil {
		return ""
	}
	return selector(query)
}

func applyExecutedPlanToDispatchResponse(response *GenerationReviewNavigationDispatchResponse, execution *GenerationNavigationDispatchExecution) {
	applyGenerationNavigationDispatchExecutionMerge(response, execution)
}

func normalizeGenerationReviewDispatchKind(target *GenerationReviewNavigationTarget) string {
	if target == nil {
		return "session"
	}
	kind := strings.ToLower(strings.TrimSpace(target.DispatchKind))
	if kind != "" {
		return kind
	}
	if target.ActionTarget != nil {
		return "action"
	}
	if target.PreviewQuery != nil && strings.TrimSpace(target.PreviewQuery.AssetID) != "" {
		return "preview"
	}
	if target.SessionQuery != nil {
		return "session"
	}
	if target.QueueQuery != nil {
		return "queue"
	}
	return "session"
}

func normalizeGenerationNavigationDispatchPlanMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "execute_plan":
		return "execute_plan"
	default:
		return "primary_only"
	}
}

func cloneGenerationReviewNavigationTarget(target *GenerationReviewNavigationTarget) *GenerationReviewNavigationTarget {
	if target == nil {
		return nil
	}
	cloned := *target
	cloned.Conditional = cloneGenerationConditionalState(target.Conditional)
	cloned.Descriptor = cloneGenerationNavigationDescriptor(target.Descriptor)
	cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)
	cloned.SessionQuery = cloneGenerationQueueQuery(target.SessionQuery)
	cloned.PreviewQuery = cloneGenerationQueueQuery(target.PreviewQuery)
	cloned.ActionTarget = cloneAssetGenerationActionTarget(target.ActionTarget)
	return applyIdentityToNavigationTarget(&cloned)
}

func finalizeGenerationReviewNavigationDispatchResponse(response *GenerationReviewNavigationDispatchResponse) *GenerationReviewNavigationDispatchResponse {
	if response == nil {
		return nil
	}
	response.PanelUpdate = buildGenerationReviewPanelUpdateFromDispatch(response)
	if (response.ReviewPreview != nil && response.ReviewPreview.NotModified) ||
		(response.Queue != nil && response.Queue.NotModified) ||
		(response.PanelUpdate != nil && response.PanelUpdate.NoChanges) {
		response.NotModified = true
	}
	return applyGenerationConditionalStateToNavigationDispatchResponse(response)
}
