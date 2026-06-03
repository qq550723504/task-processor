package listingkit

type taskGenerationActionExecuteRequestHandoffResultDispatchPhase struct {
	normalization *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase
	resultShape   *taskGenerationActionExecuteRequestHandoffResultShapePhase
}

func buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase() *taskGenerationActionExecuteRequestHandoffResultDispatchPhase {
	return &taskGenerationActionExecuteRequestHandoffResultDispatchPhase{
		normalization: buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(),
		resultShape:   buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffResultDispatchPhase) fromRetryPage(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return p.resultShape.fromRetryNormalization(p.normalization.fromRetryPage(retryPage))
}

func (p *taskGenerationActionExecuteRequestHandoffResultDispatchPhase) fromQueuePage(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return p.resultShape.fromQueueNormalization(p.normalization.fromQueuePage(queuePage))
}
