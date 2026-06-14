package listingkit

import "context"

func buildTaskStudioSessionServiceConfig(s *service) taskStudioSessionServiceConfig {
	config := buildTaskStudioSessionConfigWiring(s)
	return taskStudioSessionServiceConfig{
		repo:                     config.session.repo,
		runner:                   config.runner,
		asyncJobRunner:           config.asyncJobRunner,
		generationMetadataRunner: config.generationMetadataRunner,
		reviewTaskMetadataRunner: config.reviewTaskMetadataRunner,
		generalMetadataRunner:    config.generalMetadataRunner,
	}
}

func buildTaskStudioBatchDraftServiceConfig(s *service) taskStudioBatchDraftServiceConfig {
	config := buildTaskStudioSessionConfigWiring(s)
	return taskStudioBatchDraftServiceConfig{
		repo:   config.session.repo,
		runner: config.batchDraftRunner,
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
	return buildTaskStudioBatchServiceConfigWithCollaborators(buildTaskStudioBatchServiceConfigWiring(s))
}

func buildTaskStudioBatchServiceConfigWithCollaborators(
	config taskStudioBatchServiceConfigWiring,
) taskStudioBatchServiceConfig {
	return taskStudioBatchServiceConfig{
		repo:               config.batch.repo,
		studioSessionRepo:  config.batch.studioSessionRepo,
		generator:          config.batch.generator,
		createGenerateTask: config.batch.createGenerateTask,
		getTask:            config.batch.getTask,
		detailRunner:       config.detailRunner,
		reviewRunner:       config.reviewRunner,
		retryRunner:        config.retryRunner,
		taskPrepareRunner:  config.taskPrepare,
		taskResumeRunner:   config.taskResume,
	}
}

func buildTaskStudioBatchRunServiceConfig(s *service) taskStudioBatchRunServiceConfig {
	config := buildTaskStudioBatchRunConfigWiring(s)
	return buildTaskStudioBatchRunServiceConfigWithCollaborators(config.batchRun, s.studioBatchRunCoordinatorOrDefault())
}

func buildTaskStudioBatchRunServiceConfigWithCollaborators(
	wiring taskStudioBatchRunWiring,
	coordinator *studioBatchRunCoordinator,
) taskStudioBatchRunServiceConfig {
	var startRun func(ctx context.Context, runID string) error
	if coordinator != nil {
		startRun = coordinator.StartRun
	}
	return taskStudioBatchRunServiceConfig{
		repo:              wiring.repo,
		studioSessionRepo: wiring.studioSessionRepo,
		startRun:          startRun,
		runner:            wiring.newServiceRunner(startRun),
	}
}

func buildStudioBatchRunCoordinatorConfig(s *service) studioBatchRunCoordinatorConfig {
	config := buildTaskStudioBatchRunConfigWiring(s)
	return buildStudioBatchRunCoordinatorConfigWithCollaborators(config.batchRun, s.studioBatchRunExecutorOrDefault())
}

func buildStudioBatchRunCoordinatorConfigWithCollaborators(
	wiring taskStudioBatchRunWiring,
	executor *taskStudioBatchRunExecutor,
) studioBatchRunCoordinatorConfig {
	return studioBatchRunCoordinatorConfig{
		repo:     wiring.repo,
		executor: executor,
	}
}

func buildTaskStudioBatchRunExecutorConfig(s *service) taskStudioBatchRunExecutorConfig {
	config := buildTaskStudioBatchRunConfigWiring(s)
	return buildTaskStudioBatchRunExecutorConfigWithWiring(s, config.batchRun)
}

func buildTaskStudioBatchRunExecutorConfigWithWiring(
	s *service,
	wiring taskStudioBatchRunWiring,
) taskStudioBatchRunExecutorConfig {
	return taskStudioBatchRunExecutorConfig{
		repo:             wiring.repo,
		executeOne:       s.executeStudioBatchRunItem,
		completionRunner: wiring.newCompletionRunner(nil),
	}
}
