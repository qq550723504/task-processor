package listingkit

type taskSubmissionBindings struct {
	state     *taskSubmissionStateService
	execution *taskSubmissionExecutionService
	resolver  *submitRuntimeContextResolver
}

func buildTaskSubmissionBindings(s *service, resolver *submitRuntimeContextResolver) taskSubmissionBindings {
	if resolver == nil {
		resolver = buildSubmitRuntimeContextResolver(s)
	}
	return taskSubmissionBindings{
		state:     s.taskSubmissionStateOrDefault(),
		execution: s.taskSubmissionExecutionOrDefault(),
		resolver:  resolver,
	}
}
