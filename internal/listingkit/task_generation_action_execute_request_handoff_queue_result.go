package listingkit

type taskGenerationActionExecuteRequestHandoffQueueResultPhase struct {
	resultShape *taskGenerationActionExecuteRequestHandoffResultShapePhase
}

func buildTaskGenerationActionExecuteRequestHandoffQueueResultPhase() *taskGenerationActionExecuteRequestHandoffQueueResultPhase {
	return &taskGenerationActionExecuteRequestHandoffQueueResultPhase{
		resultShape: buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffQueueResultPhase) run(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return p.resultShape.fromQueuePage(queuePage)
}
