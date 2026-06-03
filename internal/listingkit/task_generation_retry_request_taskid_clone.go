package listingkit

func applyRetryGenerationTasksRequestTaskIDClone(req *RetryGenerationTasksRequest, cloned *RetryGenerationTasksRequest) {
	if req == nil || cloned == nil {
		return
	}
	cloned.TaskIDs = append([]string(nil), req.TaskIDs...)
}
