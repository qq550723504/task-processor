package listingkit

type taskGenerationActionExecuteRequestHandoffResultShapePhase struct{}

func buildTaskGenerationActionExecuteRequestHandoffResultShapePhase() *taskGenerationActionExecuteRequestHandoffResultShapePhase {
	return &taskGenerationActionExecuteRequestHandoffResultShapePhase{}
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
