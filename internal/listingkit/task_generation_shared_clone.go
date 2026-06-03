package listingkit

func cloneRetryGenerationTasksRequest(req *RetryGenerationTasksRequest) *RetryGenerationTasksRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	applyRetryGenerationTasksRequestCloneShape(req, &cloned)
	return &cloned
}
