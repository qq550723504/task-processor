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

func applyRetryGenerationTasksRequestCloneShape(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	applyRetryGenerationTasksRequestTaskIDClone(req, cloned)
	applyRetryGenerationTasksRequestSlotClone(req, cloned)
}

func applyRetryGenerationTasksRequestTaskIDClone(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	cloned.TaskIDs = append([]string(nil), req.TaskIDs...)
}

func applyRetryGenerationTasksRequestSlotClone(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	cloned.Slots = append([]string(nil), req.Slots...)
}
