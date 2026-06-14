package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskPreviewExportReadWiring struct {
	repo                     Repository
	listAssetGenerationTasks func(context.Context, string) ([]assetgeneration.Task, error)
}

type taskRepositoryWiring struct {
	repo Repository
}

func buildTaskRepositoryWiring(s *service) taskRepositoryWiring {
	wiring := buildServiceRepositoryWiring(s)
	return taskRepositoryWiring{
		repo: wiring.repo,
	}
}

func buildTaskPreviewExportReadWiring(s *service) taskPreviewExportReadWiring {
	repository := buildTaskRepositoryWiring(s)
	return taskPreviewExportReadWiring{
		repo: repository.repo,
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return s.listAssetGenerationTasks(ctx, taskID)
		},
	}
}

func resolveTaskSubmitter(s *service) TaskSubmitter {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.taskDeps.taskSubmitter, &s.runtime.taskSubmitter)
}

func resolveTaskRequestDefaults(s *service) generateRequestDefaults {
	if s == nil {
		return generateRequestDefaults{}
	}
	return s.taskDeps.requestDefaults
}

func resolveSDSLoginStatusProvider(s *service) SDSLoginStatusProvider {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.taskDeps.sdsLoginStatusProvider, &s.mirrors.sdsLoginStatusProvider)
}

func resolveStandardWorkflowClient(s *service) (StandardProductWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	return syncGroupedOptionalDependency(
		&s.taskDeps.standardWorkflowClient,
		&s.taskDeps.standardWorkflowEnabled,
		&s.runtime.standardProductWorkflowClient,
		&s.runtime.standardProductWorkflowEnabled,
	)
}

func resolvePlatformAdaptWorkflowClient(s *service) (PlatformAdaptWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	return syncGroupedOptionalDependency(
		&s.taskDeps.platformAdaptWorkflowClient,
		&s.taskDeps.platformAdaptWorkflowEnabled,
		&s.runtime.platformAdaptWorkflowClient,
		&s.runtime.platformAdaptWorkflowEnabled,
	)
}
