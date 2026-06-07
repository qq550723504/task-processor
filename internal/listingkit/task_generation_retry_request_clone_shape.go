package listingkit

func applyRetryGenerationTasksRequestCloneShape(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	applyRetryGenerationTasksRequestTaskIDClone(req, cloned)
	applyRetryGenerationTasksRequestSlotClone(req, cloned)
}
