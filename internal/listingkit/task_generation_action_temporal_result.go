package listingkit

import "time"

type taskGenerationActionTemporalResultPhase struct{}

func buildTaskGenerationActionTemporalResultPhase() *taskGenerationActionTemporalResultPhase {
	return &taskGenerationActionTemporalResultPhase{}
}

func (p *taskGenerationActionTemporalResultPhase) run(
	actionKey string,
	responseMode string,
	queueQuery *GenerationQueueQuery,
) *GenerationActionExecutionResult {
	resolvedTarget := &AssetGenerationActionTarget{
		ActionKey:       actionKey,
		InteractionMode: "queue_only",
	}
	if queueQuery != nil {
		resolvedTarget.QueueQuery = cloneGenerationQueueQuery(queueQuery)
	}

	return &GenerationActionExecutionResult{
		ActionKey:       actionKey,
		InteractionMode: "queue_only",
		ResponseMode:    normalizeGenerationActionResponseMode(responseMode),
		ResolvedTarget:  resolvedTarget,
		Audit: &GenerationActionAudit{
			RequestedActionKey: actionKey,
			ResolvedActionKey:  actionKey,
			ResolutionSource:   "layer_temporal",
			ExecutionPath:      "queue_only",
			ExecutedAt:         time.Now().UTC(),
		},
	}
}
