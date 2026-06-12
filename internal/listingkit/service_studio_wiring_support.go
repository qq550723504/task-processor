package listingkit

type taskStudioSessionRepoWiring struct {
	repo StudioSessionRepository
}

func buildTaskStudioSessionRepoWiring(s *service) taskStudioSessionRepoWiring {
	return taskStudioSessionRepoWiring{
		repo: s.studioSessionRepo,
	}
}
