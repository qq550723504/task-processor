package listingkit

type taskGenerationActionExecuteRequestHandoffRetryResultPhase struct {
	adaptation *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase
}

func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase() *taskGenerationActionExecuteRequestHandoffRetryResultPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryResultPhase{
		adaptation: buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffRetryResultPhase) run(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return p.adaptation.fromRetryPage(retryPage)
}
