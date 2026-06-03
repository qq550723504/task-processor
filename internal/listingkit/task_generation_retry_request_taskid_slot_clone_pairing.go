package listingkit

func applyRetryGenerationTasksRequestTaskIDSlotClonePairing(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	applyRetryGenerationTasksRequestTaskIDClone(req, cloned)
	applyRetryGenerationTasksRequestSlotClone(req, cloned)
}
