package listingkit

func cloneRetryGenerationTasksRequest(req *RetryGenerationTasksRequest) *RetryGenerationTasksRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	cloned.TaskIDs = append([]string(nil), req.TaskIDs...)
	cloned.Slots = append([]string(nil), req.Slots...)
	return &cloned
}
