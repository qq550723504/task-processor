package listingkit

type taskSubmissionOrchestratorWiring struct {
	lockSubmit func(string) func()
	recovery   *taskSubmissionRecoveryService
	bindings   taskSubmissionBindings
}

func buildTaskSubmissionLockSubmit(s *service) func(string) func() {
	return func(key string) func() {
		return s.submission.sheinSubmitLocks.Lock(key)
	}
}

func buildTaskSubmissionOrchestratorWiring(s *service, resolver *submitRuntimeContextResolver) taskSubmissionOrchestratorWiring {
	return taskSubmissionOrchestratorWiring{
		lockSubmit: buildTaskSubmissionLockSubmit(s),
		recovery:   s.taskSubmissionRecoveryOrDefault(),
		bindings:   buildTaskSubmissionBindings(s, resolver),
	}
}
