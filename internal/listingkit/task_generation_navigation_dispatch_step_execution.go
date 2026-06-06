package listingkit

import (
	"context"
	"errors"
	"strings"
)

type taskGenerationNavigationDispatchStepExecutionPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationNavigationDispatchStepExecutionPhase(service *taskGenerationService) *taskGenerationNavigationDispatchStepExecutionPhase {
	return &taskGenerationNavigationDispatchStepExecutionPhase{service: service}
}

func (p *taskGenerationNavigationDispatchStepExecutionPhase) runSequential(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	if p == nil || p.service == nil || plan == nil || execution == nil {
		return
	}

	for index, step := range plan.Steps {
		stepResult := p.run(ctx, taskID, step, responseMode)
		if stepResult == nil {
			continue
		}
		execution.Steps = append(execution.Steps, *stepResult)
		applyGenerationNavigationDispatchExecutionStats(execution, stepResult)
		if stepResult.Status == "failed" && plan.StopOnError {
			execution.StopReason = "error"
		}
		if execution.StopReason == "" && shouldStopGenerationNavigationDispatchPlan(plan, stepResult) {
			execution.StopReason = generationNavigationDispatchPlanStopReason(plan, stepResult)
		}
		if execution.StopReason != "" {
			p.backfillSkippedSteps(plan, execution, index+1)
			return
		}
	}
}

func (p *taskGenerationNavigationDispatchStepExecutionPhase) run(ctx context.Context, taskID string, step GenerationNavigationDispatchStep, responseMode string) *GenerationNavigationDispatchExecutionStep {
	if p == nil || p.service == nil {
		return nil
	}

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
		queue, err := p.service.GetTaskGenerationQueue(ctx, taskID, query)
		if err != nil {
			return p.failedResult(result, err)
		}
		result.Queue = queue
		deltaToken, notModified := queueReadState(queue)
		p.applyReadState(result, deltaToken, notModified)
	case "preview":
		preview, err := p.service.GetTaskGenerationReviewPreview(ctx, taskID, query)
		if err != nil {
			return p.failedResult(result, err)
		}
		result.ReviewPreview = preview
		deltaToken, notModified := previewReadState(preview)
		p.applyReadState(result, deltaToken, notModified)
	default:
		session, err := p.service.GetTaskGenerationReviewSession(ctx, taskID, query)
		if err != nil {
			return p.failedResult(result, err)
		}
		result.ReviewSession = session
		deltaToken, notModified := sessionReadState(session)
		p.applyReadState(result, deltaToken, notModified)
	}

	return result
}

func (p *taskGenerationNavigationDispatchStepExecutionPhase) backfillSkippedSteps(plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution, start int) {
	if plan == nil || execution == nil {
		return
	}
	for remaining := start; remaining < len(plan.Steps); remaining++ {
		next := plan.Steps[remaining]
		skipped := generationNavigationDispatchPlanSkippedStep(next, execution.StopReason)
		execution.Steps = append(execution.Steps, skipped)
		applyGenerationNavigationDispatchExecutionStats(execution, &skipped)
	}
}

func (p *taskGenerationNavigationDispatchStepExecutionPhase) failedResult(result *GenerationNavigationDispatchExecutionStep, err error) *GenerationNavigationDispatchExecutionStep {
	if result == nil || err == nil {
		return result
	}
	result.Status = "failed"
	result.Error = err.Error()
	result.ErrorKind = classifyGenerationNavigationDispatchStepError(err)
	return result
}

func classifyGenerationNavigationDispatchStepError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrTaskNotFound), errors.Is(err, ErrGenerationTaskNotFound), errors.Is(err, ErrGenerationActionNotFound):
		return "not_found"
	case errors.Is(err, ErrTaskNotPending), errors.Is(err, ErrGenerationTaskNotRetryable):
		return "conflict"
	default:
		return "internal"
	}
}

func (p *taskGenerationNavigationDispatchStepExecutionPhase) applyReadState(result *GenerationNavigationDispatchExecutionStep, deltaToken string, notModified bool) {
	if result == nil {
		return
	}
	result.DeltaToken = deltaToken
	result.NotModified = notModified
	result.NoChanges = notModified
	if notModified {
		result.Status = "not_modified"
	}
}

func queueReadState(queue *GenerationQueuePage) (string, bool) {
	if queue == nil {
		return "", false
	}
	return queue.DeltaToken, queue.NotModified
}

func previewReadState(preview *GenerationReviewPreviewResponse) (string, bool) {
	if preview == nil {
		return "", false
	}
	return preview.DeltaToken, preview.NotModified
}

func sessionReadState(session *GenerationReviewSessionResponse) (string, bool) {
	if session == nil {
		return "", false
	}
	return session.DeltaToken, session.NotModified
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
