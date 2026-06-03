package listingkit

type taskGenerationActionExecuteRequestHandoffResultNormalization struct {
	retryPage        *GenerationTaskPage
	queuePage        *GenerationQueuePage
	persistenceQueue *GenerationWorkQueue
}

type taskGenerationActionExecuteRequestHandoffResultNormalizationPhase struct {
	adaptation *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase
}

func buildTaskGenerationActionExecuteRequestHandoffResultNormalizationPhase() *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase {
	return &taskGenerationActionExecuteRequestHandoffResultNormalizationPhase{
		adaptation: buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase) fromRetryPage(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoffResultNormalization {
	return &taskGenerationActionExecuteRequestHandoffResultNormalization{
		retryPage:        retryPage,
		persistenceQueue: p.adaptation.persistenceQueueFromRetryPage(retryPage),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffResultNormalizationPhase) fromQueuePage(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoffResultNormalization {
	return &taskGenerationActionExecuteRequestHandoffResultNormalization{
		queuePage:        queuePage,
		persistenceQueue: p.adaptation.persistenceQueueFromQueuePage(queuePage),
	}
}
