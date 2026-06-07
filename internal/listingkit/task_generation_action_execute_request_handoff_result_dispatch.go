package listingkit

type taskGenerationActionExecuteRequestHandoffResultDispatchPhase struct {
	normalization *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase
	resultShape   *taskGenerationActionExecuteRequestHandoffResultShapePhase
}

type taskGenerationActionExecuteRequestHandoffResultShapePhase struct{}

func buildTaskGenerationActionExecuteRequestHandoffResultDispatchPhase() *taskGenerationActionExecuteRequestHandoffResultDispatchPhase {
	return &taskGenerationActionExecuteRequestHandoffResultDispatchPhase{
		normalization: buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase(),
		resultShape:   buildTaskGenerationActionExecuteRequestHandoffResultShapePhase(),
	}
}

func buildTaskGenerationActionExecuteRequestHandoffResultShapePhase() *taskGenerationActionExecuteRequestHandoffResultShapePhase {
	return &taskGenerationActionExecuteRequestHandoffResultShapePhase{}
}

func (p *taskGenerationActionExecuteRequestHandoffResultDispatchPhase) fromRetryPage(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return p.resultShape.fromRetryNormalization(p.normalization.fromRetryPage(retryPage))
}

func (p *taskGenerationActionExecuteRequestHandoffResultDispatchPhase) fromQueuePage(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return p.resultShape.fromQueueNormalization(p.normalization.fromQueuePage(queuePage))
}

func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromRetryNormalization(normalized *taskGenerationActionExecuteRequestHandoffResultNormalization) *taskGenerationActionExecuteRequestHandoff {
	return &taskGenerationActionExecuteRequestHandoff{
		retryPage:        normalized.retryPage,
		persistenceQueue: normalized.persistenceQueue,
	}
}

func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromQueueNormalization(normalized *taskGenerationActionExecuteRequestHandoffResultNormalization) *taskGenerationActionExecuteRequestHandoff {
	return &taskGenerationActionExecuteRequestHandoff{
		queuePage:        normalized.queuePage,
		persistenceQueue: normalized.persistenceQueue,
	}
}
