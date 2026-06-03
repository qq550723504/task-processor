package listingkit

func applyRetryGenerationTasksRequestSliceClone(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	applyRetryGenerationTasksRequestTaskIDSlotClonePairing(req, cloned)
}
