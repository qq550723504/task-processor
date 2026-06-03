package listingkit

type taskGenerationActionExecuteRequestHandoffQueueResultPhase struct {
	adaptation *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase
}

func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase() *taskGenerationActionExecuteRequestHandoffQueueResultPhase {
	return &taskGenerationActionExecuteRequestHandoffQueueResultPhase{
		adaptation: buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return p.adaptation.fromQueuePage(queuePage)
}
