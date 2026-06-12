package listingkit

type taskSubmissionOrchestratorWiring struct {
	lockSubmit func(string) func()
	recovery   *taskSubmissionRecoveryService
	bindings   taskSubmissionBindings
}

type taskSubmitterWiring struct {
	repo          Repository
	taskSubmitter func() TaskSubmitter
}

func buildTaskSubmitterWiring(s *service) taskSubmitterWiring {
	return taskSubmitterWiring{
		repo: s.repo,
		taskSubmitter: func() TaskSubmitter {
			return resolveTaskSubmitter(s)
		},
	}
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
