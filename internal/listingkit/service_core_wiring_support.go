package listingkit

import "context"

type serviceRepositoryWiring struct {
	repo    Repository
	getTask func(context.Context, string) (*Task, error)
}

func buildServiceRepositoryWiring(s *service) serviceRepositoryWiring {
	if s == nil {
		return serviceRepositoryWiring{}
	}
	wiring := serviceRepositoryWiring{repo: s.repo}
	if s.repo != nil {
		wiring.getTask = s.repo.GetTask
	}
	return wiring
}
