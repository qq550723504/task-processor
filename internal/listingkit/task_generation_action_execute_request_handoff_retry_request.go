package listingkit

type taskGenerationActionExecuteRequestHandoffRetryRequestPhase struct{}

func buildTaskGenerationActionExecuteRequestHandoffRetryRequestPhase() *taskGenerationActionExecuteRequestHandoffRetryRequestPhase {
	return &taskGenerationActionExecuteRequestHandoffRetryRequestPhase{}
}

func (p *taskGenerationActionExecuteRequestHandoffRetryRequestPhase) run(target *AssetGenerationActionTarget) *RetryGenerationTasksRequest {
	if target == nil {
		return nil
	}
	return cloneRetryGenerationTasksRequest(target.RetryRequest)
}
