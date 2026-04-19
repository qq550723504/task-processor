package listingkit

func cloneGenerationWorkQueue(queue *GenerationWorkQueue) *GenerationWorkQueue {
	if queue == nil {
		return nil
	}
	cloned := &GenerationWorkQueue{
		Items: append([]GenerationWorkQueueItem(nil), queue.Items...),
	}
	if queue.Summary != nil {
		summary := *queue.Summary
		cloned.Summary = &summary
	}
	return cloned
}
