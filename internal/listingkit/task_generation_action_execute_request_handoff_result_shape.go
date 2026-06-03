package listingkit

type taskGenerationActionExecuteRequestHandoffResultShapePhase struct {
	adaptation *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase
}

func buildTaskGenerationActionExecuteRequestHandoffResultShapePhase() *taskGenerationActionExecuteRequestHandoffResultShapePhase {
	return &taskGenerationActionExecuteRequestHandoffResultShapePhase{
		adaptation: buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase(),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromRetryPage(retryPage *GenerationTaskPage) *taskGenerationActionExecuteRequestHandoff {
	return &taskGenerationActionExecuteRequestHandoff{
		retryPage:        retryPage,
		persistenceQueue: p.adaptation.persistenceQueueFromRetryPage(retryPage),
	}
}

func (p *taskGenerationActionExecuteRequestHandoffResultShapePhase) fromQueuePage(queuePage *GenerationQueuePage) *taskGenerationActionExecuteRequestHandoff {
	return &taskGenerationActionExecuteRequestHandoff{
		queuePage:        queuePage,
		persistenceQueue: p.adaptation.persistenceQueueFromQueuePage(queuePage),
	}
}
