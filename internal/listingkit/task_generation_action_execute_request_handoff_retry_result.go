package listingkit

type taskGenerationActionExecuteRequestHandoffRetryResultPhase struct {
	resultShape *taskGenerationActionExecuteRequestHandoffResultShapePhase
}

func buildTaskGenerationActionExecuteRequestHandoffRetryResultPhase() *taskGenerationActionExecuteRequestHandoffRetryResultPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryResultPhase{
		resultShape: buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffRetryResultPhase) run(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return p.resultShape.fromRetryPage(retryPage)
}
