package listingkit

type taskRepositoryWiring struct {
	repo Repository
}

func buildTaskRepositoryWiring(s *service) taskRepositoryWiring {
	wiring := buildServiceRepositoryWiring(s)
	return taskRepositoryWiring{
		repo: wiring.repo,
	}
}

func resolveTaskSubmitter(s *service) TaskSubmitter {
	if s == nil {
		return nil
	}
	return s.taskDeps.taskSubmitter
}

func resolveSDSLoginStatusProvider(s *service) SDSLoginStatusProvider {
	if s == nil {
		return nil
	}
	return s.taskDeps.sdsLoginStatusProvider
}

func resolveStandardWorkflowClient(s *service) (StandardProductWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	return s.taskDeps.standardWorkflowClient, s.taskDeps.standardWorkflowEnabled
}

func resolvePlatformAdaptWorkflowClient(s *service) (PlatformAdaptWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	return s.taskDeps.platformAdaptWorkflowClient, s.taskDeps.platformAdaptWorkflowEnabled
}
