package listingkit

import "context"

func buildTaskStudioSessionServiceConfig(s *service) taskStudioSessionServiceConfig {
	wiring := buildTaskStudioSessionRepoWiring(s)
	return taskStudioSessionServiceConfig{
		repo:                     wiring.repo,
		runner:                   newListingStudioSessionService(wiring.repo),
		asyncJobRunner:           newListingStudioSessionAsyncJobService(wiring.repo),
		generationMetadataRunner: newListingStudioSessionGenerationMetadataService(wiring.repo),
		reviewTaskMetadataRunner: newListingStudioSessionReviewTaskMetadataService(wiring.repo),
		generalMetadataRunner:    newListingStudioSessionGeneralMetadataService(wiring.repo),
	}
}

func buildTaskStudioBatchDraftServiceConfig(s *service) taskStudioBatchDraftServiceConfig {
	wiring := buildTaskStudioSessionRepoWiring(s)
	return taskStudioBatchDraftServiceConfig{
		repo:   wiring.repo,
		runner: newListingStudioBatchDraftService(wiring.repo),
	}
}

func buildTaskStudioMediaServiceConfig(s *service) taskStudioMediaServiceConfig {
	wiring := buildTaskStudioMediaWiring(s)
	return taskStudioMediaServiceConfig{
		imageGenerator:        wiring.imageGenerator,
		promptDiversifier:     wiring.promptDiversifier,
		uploadStoreConfigured: wiring.uploadStoreConfigured,
		uploadImages:          wiring.uploadImages,
	}
}

func buildStudioBatchGenerationServiceConfig(s *service) studioBatchGenerationServiceConfig {
	wiring := buildStudioBatchGenerationWiring(s)
	return studioBatchGenerationServiceConfig{
		repo:    wiring.repo,
		execute: wiring.execute,
	}
}

func buildTaskStudioBatchServiceConfig(s *service) taskStudioBatchServiceConfig {
	wiring := buildTaskStudioBatchServiceWiring(s)
	return taskStudioBatchServiceConfig{
		repo:               wiring.repo,
		studioSessionRepo:  wiring.studioSessionRepo,
		generator:          wiring.generator,
		createGenerateTask: wiring.createGenerateTask,
		getTask:            wiring.getTask,
		detailRunner:       wiring.detailRunner,
		reviewRunner:       wiring.reviewRunner,
	}
}

func buildTaskStudioBatchRunServiceConfig(s *service) taskStudioBatchRunServiceConfig {
	wiring := buildTaskStudioBatchRunRepoWiring(s)
	var startRun func(ctx context.Context, runID string) error
	if s.buildStudioBatchRunCoordinator() != nil {
		startRun = s.startStudioBatchRun
	}
	return taskStudioBatchRunServiceConfig{
		repo:              wiring.repo,
		studioSessionRepo: wiring.studioSessionRepo,
		startRun:          startRun,
		runner:            newListingStudioBatchRunService(wiring.repo, wiring.studioSessionRepo, startRun),
	}
}

func buildStudioBatchRunCoordinatorConfig(s *service) studioBatchRunCoordinatorConfig {
	wiring := buildTaskStudioBatchRunRepoWiring(s)
	return studioBatchRunCoordinatorConfig{
		repo:     wiring.repo,
		executor: s.buildStudioBatchRunExecutor(),
	}
}

func buildTaskStudioBatchRunExecutorConfig(s *service) taskStudioBatchRunExecutorConfig {
	wiring := buildTaskStudioBatchRunRepoWiring(s)
	return taskStudioBatchRunExecutorConfig{
		repo:             wiring.repo,
		executeOne:       s.executeStudioBatchRunItem,
		completionRunner: newListingStudioBatchRunCompletionService(wiring.repo, nil),
	}
}
