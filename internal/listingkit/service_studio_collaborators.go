package listingkit

func (s *service) taskStudioSessionOrDefault() *taskStudioSessionService {
	return syncGroupedCollaborator(&s.studio.session, &s.collabMirrors.taskStudioSession, func() *taskStudioSessionService {
		return newTaskStudioSessionService(buildTaskStudioSessionServiceConfig(s))
	})
}

func (s *service) taskStudioBatchDraftOrDefault() *taskStudioBatchDraftService {
	return syncGroupedCollaborator(&s.studio.batchDraft, &s.collabMirrors.taskStudioBatchDraft, func() *taskStudioBatchDraftService {
		return newTaskStudioBatchDraftService(buildTaskStudioBatchDraftServiceConfig(s))
	})
}

func (s *service) taskStudioMediaOrDefault() *taskStudioMediaService {
	return syncGroupedCollaborator(&s.studio.media, &s.collabMirrors.taskStudioMedia, func() *taskStudioMediaService {
		return newTaskStudioMediaService(buildTaskStudioMediaServiceConfig(s))
	})
}

func (s *service) studioBatchGenerationOrDefault() *studioBatchGenerationService {
	return syncGroupedCollaborator(&s.studio.batchGeneration, &s.collabMirrors.studioBatchGeneration, func() *studioBatchGenerationService {
		return newStudioBatchGenerationService(buildStudioBatchGenerationServiceConfig(s))
	})
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	return syncGroupedCollaborator(&s.studio.batch, &s.collabMirrors.taskStudioBatch, func() *taskStudioBatchService {
		return newTaskStudioBatchService(buildTaskStudioBatchServiceConfig(s))
	})
}

func (s *service) taskStudioBatchRunOrDefault() *taskStudioBatchRunService {
	return syncGroupedCollaborator(&s.studio.batchRun, &s.collabMirrors.taskStudioBatchRun, func() *taskStudioBatchRunService {
		return newTaskStudioBatchRunService(buildTaskStudioBatchRunServiceConfig(s))
	})
}

func (s *service) studioBatchRunExecutorOrDefault() *taskStudioBatchRunExecutor {
	return syncGroupedCollaborator(&s.studio.runExecutor, &s.collabMirrors.studioBatchRunExecutor, nil)
}

func (s *service) studioBatchRunCoordinatorOrDefault() *studioBatchRunCoordinator {
	return syncGroupedCollaborator(&s.studio.runCoordinator, &s.collabMirrors.studioBatchRunCoordinator, nil)
}
