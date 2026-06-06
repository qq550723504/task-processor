package listingkit

type taskGenerationActionExecuteRequestHandoffResultAdaptationPhase struct{}

func buildTaskGenerationActionExecuteRequestHandoffResultAdaptationPhase() *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase {
	return &taskGenerationActionExecuteRequestHandoffResultAdaptationPhase{}
}

func generationWorkQueueFromPage(page *GenerationQueuePage) *GenerationWorkQueue {
	if page == nil {
		return nil
	}
	return &GenerationWorkQueue{
		Summary: page.Summary,
		Items:   append([]GenerationWorkQueueItem(nil), page.Items...),
	}
}

func generationWorkQueueFromRetryPage(page *GenerationTaskPage) *GenerationWorkQueue {
	if page == nil {
		return nil
	}
	if page.ExecutedQueue != nil {
		return page.ExecutedQueue
	}
	if page.MatchedQueue != nil {
		return page.MatchedQueue
	}
	return nil
}

func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) persistenceQueueFromRetryPage(retryPage *GenerationTaskPage) *GenerationWorkQueue {
	return generationWorkQueueFromRetryPage(retryPage)
}

func (p *taskGenerationActionExecuteRequestHandoffResultAdaptationPhase) persistenceQueueFromQueuePage(queuePage *GenerationQueuePage) *GenerationWorkQueue {
	return generationWorkQueueFromPage(queuePage)
}
