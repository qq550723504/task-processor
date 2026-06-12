package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskPreviewExportReadWiring struct {
	repo                     Repository
	listAssetGenerationTasks func(context.Context, string) ([]assetgeneration.Task, error)
}

func buildTaskPreviewExportReadWiring(s *service) taskPreviewExportReadWiring {
	return taskPreviewExportReadWiring{
		repo: s.repo,
		listAssetGenerationTasks: func(ctx context.Context, taskID string) ([]assetgeneration.Task, error) {
			return s.listAssetGenerationTasks(ctx, taskID)
		},
	}
}

func resolveTaskSubmitter(s *service) TaskSubmitter {
	if s == nil {
		return nil
	}
	if s.taskDeps.taskSubmitter != nil {
		s.taskSubmitter = s.taskDeps.taskSubmitter
		return s.taskDeps.taskSubmitter
	}
	s.taskDeps.taskSubmitter = s.taskSubmitter
	return s.taskSubmitter
}

func resolveTaskRequestDefaults(s *service) generateRequestDefaults {
	if s == nil {
		return generateRequestDefaults{}
	}
	if s.taskDeps.requestDefaults != (generateRequestDefaults{}) {
		s.requestDefaults = s.taskDeps.requestDefaults
		return s.taskDeps.requestDefaults
	}
	s.taskDeps.requestDefaults = s.requestDefaults
	return s.requestDefaults
}

func resolveSDSLoginStatusProvider(s *service) SDSLoginStatusProvider {
	if s == nil {
		return nil
	}
	if s.taskDeps.sdsLoginStatusProvider != nil {
		s.sdsLoginStatusProvider = s.taskDeps.sdsLoginStatusProvider
		return s.taskDeps.sdsLoginStatusProvider
	}
	s.taskDeps.sdsLoginStatusProvider = s.sdsLoginStatusProvider
	return s.sdsLoginStatusProvider
}

func resolveStandardWorkflowClient(s *service) (StandardProductWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	if s.taskDeps.standardWorkflowClient != nil || s.taskDeps.standardWorkflowEnabled {
		s.standardProductWorkflowClient = s.taskDeps.standardWorkflowClient
		s.standardProductWorkflowEnabled = s.taskDeps.standardWorkflowEnabled
		return s.taskDeps.standardWorkflowClient, s.taskDeps.standardWorkflowEnabled
	}
	s.taskDeps.standardWorkflowClient = s.standardProductWorkflowClient
	s.taskDeps.standardWorkflowEnabled = s.standardProductWorkflowEnabled
	return s.standardProductWorkflowClient, s.standardProductWorkflowEnabled
}

func resolvePlatformAdaptWorkflowClient(s *service) (PlatformAdaptWorkflowClient, bool) {
	if s == nil {
		return nil, false
	}
	if s.taskDeps.platformAdaptWorkflowClient != nil || s.taskDeps.platformAdaptWorkflowEnabled {
		s.platformAdaptWorkflowClient = s.taskDeps.platformAdaptWorkflowClient
		s.platformAdaptWorkflowEnabled = s.taskDeps.platformAdaptWorkflowEnabled
		return s.taskDeps.platformAdaptWorkflowClient, s.taskDeps.platformAdaptWorkflowEnabled
	}
	s.taskDeps.platformAdaptWorkflowClient = s.platformAdaptWorkflowClient
	s.taskDeps.platformAdaptWorkflowEnabled = s.platformAdaptWorkflowEnabled
	return s.platformAdaptWorkflowClient, s.platformAdaptWorkflowEnabled
}
