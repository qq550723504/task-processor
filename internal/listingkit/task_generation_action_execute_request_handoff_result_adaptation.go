package listingkit

type taskGenerationActionExecuteRequestHandoffResultAdaptationPhase struct{}

func buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase() *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase {
	return &taskGenerationActionExecuteRequestHandoffResultAdaptationPhase{}
}

func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) fromRetryPage(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return &taskGenerationActionExecuteRequestHandoff{
		retryPage:        retryPage,
		persistenceQueue: generationWorkQueueFromRetryPage(retryPage),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) fromQueuePage(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return &taskGenerationActionExecuteRequestHandoff{
		queuePage:        queuePage,
		persistenceQueue: generationWorkQueueFromPage(queuePage),
	}
}
