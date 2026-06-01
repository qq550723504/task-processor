package listingkit

import "context"

type taskGenerationActionPersistPhase struct {
	service *taskGenerationService
}

func buildTaskGenerationActionPersistPhase(service *taskGenerationService) *taskGenerationActionPersistPhase {
	return &taskGenerationActionPersistPhase{service: service}
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
