package listingkit

import (
	"context"
	"strings"
	"sync"
)

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
