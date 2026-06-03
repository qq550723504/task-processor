package listingkit

func applyRetryGenerationTasksRequestSlotClone(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	cloned.Slots = append([]string(nil), req.Slots...)
}
