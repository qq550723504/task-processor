package listingkit

import "context"

func buildTaskStudioSessionServiceConfig(s *service) taskStudioSessionServiceConfig {
	return taskStudioSessionServiceConfig{
		repo: s.studioSessionRepo,
	}
}

func buildTaskStudioBatchDraftServiceConfig(s *service) taskStudioBatchDraftServiceConfig {
	return taskStudioBatchDraftServiceConfig{
		repo: s.studioSessionRepo,
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
		repo:              s.studioBatchRepo,
		studioSessionRepo: s.studioSessionRepo,
		generator:         newStudioBatchGenerationService(buildStudioBatchGenerationServiceConfig(s)),
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
