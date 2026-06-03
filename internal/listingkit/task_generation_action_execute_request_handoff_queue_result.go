package listingkit

type taskGenerationActionExecuteRequestHandoffQueueResultPhase struct {
	dispatch *taskGenerationActionExecuteRequestHandoffResultDispatchPhase
}

func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase() *taskGenerationActionExecuteRequestHandoffQueueResultPhase {
	return &taskGenerationActionExecuteRequestHandoffQueueResultPhase{
		dispatch: buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return p.dispatch.fromQueuePage(queuePage)
}
