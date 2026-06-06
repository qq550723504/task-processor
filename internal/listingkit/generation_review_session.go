package listingkit

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
