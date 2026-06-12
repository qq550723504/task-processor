package listingkit

type taskStudioSessionRepoWiring struct {
	repo StudioSessionRepository
}

type taskStudioBatchRunRepoWiring struct {
	repo              StudioBatchRunRepository
	studioSessionRepo StudioSessionRepository
}

func buildTaskStudioSessionRepoWiring(s *service) taskStudioSessionRepoWiring {
	return taskStudioSessionRepoWiring{
		repo: s.studioSessionRepo,
	}
}

func buildTaskStudioBatchRunRepoWiring(s *service) taskStudioBatchRunRepoWiring {
	return taskStudioBatchRunRepoWiring{
		repo:              s.studioBatchRunRepo,
		studioSessionRepo: s.studioSessionRepo,
	}
}
