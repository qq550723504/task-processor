package listingkit

import "context"

func buildTaskStudioSessionServiceConfig(s *service) taskStudioSessionServiceConfig {
	wiring := buildTaskStudioSessionRepoWiring(s)
	return taskStudioSessionServiceConfig{
		repo: wiring.repo,
	}
}

func buildTaskStudioBatchDraftServiceConfig(s *service) taskStudioBatchDraftServiceConfig {
	wiring := buildTaskStudioSessionRepoWiring(s)
	return taskStudioBatchDraftServiceConfig{
		repo: wiring.repo,
	}
}

func buildTaskStudioMediaServiceConfig(s *service) taskStudioMediaServiceConfig {
	return taskStudioMediaServiceConfig{
		imageGenerator:        s.studioImageGenerator,
		promptDiversifier:     s.studioPromptDiversifier,
		uploadStoreConfigured: s.uploadStore != nil,
		uploadImages:          s.UploadImages,
	}
}

func buildStudioBatchGenerationServiceConfig(s *service) studioBatchGenerationServiceConfig {
	return studioBatchGenerationServiceConfig{
		repo: s.studioBatchRepo,
		execute: func(ctx context.Context, input StudioBatchGenerateExecutionInput) (*StudioBatchGenerateExecutionOutput, error) {
			return ExecuteStudioDesignBatch(ctx, s, input)
		},
	}
}

func buildTaskStudioBatchServiceConfig(s *service) taskStudioBatchServiceConfig {
	if s == nil {
		return taskStudioBatchServiceConfig{}
	}
	var getTask func(context.Context, string) (*Task, error)
	if s.repo != nil {
		getTask = s.repo.GetTask
	}
	return taskStudioBatchServiceConfig{
		repo:               s.studioBatchRepo,
		studioSessionRepo:  s.studioSessionRepo,
		generator:          s.studioBatchGenerationOrDefault(),
		createGenerateTask: s.CreateGenerateTask,
		getTask:            getTask,
	}
}

func buildTaskStudioBatchRunServiceConfig(s *service) taskStudioBatchRunServiceConfig {
	var startRun func(ctx context.Context, runID string) error
	if s.buildStudioBatchRunCoordinator() != nil {
		startRun = s.startStudioBatchRun
	}
	return taskStudioBatchRunServiceConfig{
		repo:              s.studioBatchRunRepo,
		studioSessionRepo: s.studioSessionRepo,
		startRun:          startRun,
	}
}

func buildStudioBatchRunCoordinatorConfig(s *service) studioBatchRunCoordinatorConfig {
	return studioBatchRunCoordinatorConfig{
		repo:     s.studioBatchRunRepo,
		executor: s.buildStudioBatchRunExecutor(),
	}
}

func buildTaskStudioBatchRunExecutorConfig(s *service) taskStudioBatchRunExecutorConfig {
	return taskStudioBatchRunExecutorConfig{
		repo:       s.studioBatchRunRepo,
		executeOne: s.executeStudioBatchRunItem,
	}
}
