package listingkit

func cloneGenerationQueueQuery(query *GenerationQueueQuery) *GenerationQueueQuery {
	if query == nil {
		return nil
	}
	cloned := *query
	return &cloned
}

func cloneRetryGenerationTasksRequest(req *RetryGenerationTasksRequest) *RetryGenerationTasksRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	applyRetryGenerationTasksRequestCloneShape(req, &cloned)
	return &cloned
}
