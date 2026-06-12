package listingkit

import "context"

type taskStudioSessionRepoWiring struct {
	repo StudioSessionRepository
}

type taskStudioBatchServiceWiring struct {
	repo               StudioBatchRepository
	studioSessionRepo  StudioSessionRepository
	generator          *studioBatchGenerationService
	createGenerateTask func(context.Context, *GenerateRequest) (*Task, error)
	getTask            func(context.Context, string) (*Task, error)
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

func buildTaskStudioBatchServiceWiring(s *service) taskStudioBatchServiceWiring {
	if s == nil {
		return taskStudioBatchServiceWiring{}
	}
	var getTask func(context.Context, string) (*Task, error)
	if s.repo != nil {
		getTask = s.repo.GetTask
	}
	return taskStudioBatchServiceWiring{
		repo:               s.studioBatchRepo,
		studioSessionRepo:  s.studioSessionRepo,
		generator:          s.studioBatchGenerationOrDefault(),
		createGenerateTask: s.CreateGenerateTask,
		getTask:            getTask,
	}
}
