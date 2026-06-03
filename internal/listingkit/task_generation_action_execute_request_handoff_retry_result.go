package listingkit

type taskGenerationActionExecuteRequestHandoffRetryResultPhase struct {
	dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase
}

func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase() *taskGenerationActionExecuteRequestHandoffRetryResultPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryResultPhase{
		dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffRetryResultPhase) run(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return p.dispatch.fromRetryPage(retryPage)
}
