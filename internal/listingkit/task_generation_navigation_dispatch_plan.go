package listingkit

import "context"

type taskGenerationNavigationDispatchPlanPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationNavigationDispatchPlanPhase(service *taskGenerationService) *taskGenerationNavigationDispatchPlanPhase {
	return &taskGenerationNavigationDispatchPlanPhase{service: service}
}

func (p *taskGenerationNavigationDispatchPlanPhase) run(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {
	if p == nil || p.service == nil || target == nil || target.Descriptor == nil || target.Descriptor.DispatchPlan == nil {
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
		p.service.executeGenerationNavigationDispatchPlanParallel(ctx, taskID, responseMode, plan, execution)
		applyGenerationNavigationDispatchExecutionRules(plan, execution)
		return execution, nil
	}

	p.service.executeGenerationNavigationDispatchPlanSequential(ctx, taskID, responseMode, plan, execution)
	applyGenerationNavigationDispatchExecutionRules(plan, execution)
	return execution, nil
}
