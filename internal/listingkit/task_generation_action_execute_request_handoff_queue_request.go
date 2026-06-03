package listingkit

type taskGenerationActionExecuteRequestHandoffQueueRequestPhase struct{}

func buildTaskGenerationActionExecuteRequestHandoffQueueRequestPhase() *taskGenerationActionExecuteRequestHandoffQueueRequestPhase {
	return &taskGenerationActionExecuteRequestHandoffQueueRequestPhase{}
}

func (p *taskGenerationActionExecuteRequestHandoffQueueRequestPhase) run(target *AssetGenerationActionTarget) *GenerationQueueQuery {
	if target == nil {
		return nil
	}
	return cloneGenerationQueueQuery(target.QueueQuery)
}
