package listingkit

func (s *service) taskStudioSessionOrDefault() *taskStudioSessionService {
	return s.resolveTaskStudioSessionCollaborators().session
}

func (s *service) taskStudioBatchDraftOrDefault() *taskStudioBatchDraftService {
	return s.resolveTaskStudioSessionCollaborators().batchDraft
}

func (s *service) taskStudioMediaOrDefault() *taskStudioMediaService {
	return s.resolveTaskStudioSessionCollaborators().media
}

func (s *service) resolveTaskStudioSessionCollaborators() taskStudioSessionCollaborators {
	if s == nil {
		return taskStudioSessionCollaborators{}
	}
	wiring := buildTaskStudioSessionCollaboratorWiring(s)
	s.studio.sessionGroup = wiring.resolve(s.studio.sessionGroup)
	return s.studio.sessionGroup
}

func (s *service) studioBatchGenerationOrDefault() *studioBatchGenerationService {
	return s.resolveTaskStudioBatchCollaborators().batchGeneration
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	return s.resolveTaskStudioBatchCollaborators().batch
}

func (s *service) resolveTaskStudioBatchCollaborators() taskStudioBatchCollaborators {
	if s == nil {
		return taskStudioBatchCollaborators{}
	}
	wiring := buildTaskStudioBatchCollaboratorWiring(s)
	s.studio.batchGroup = wiring.resolve(s.studio.batchGroup)
	return s.studio.batchGroup
}

func (s *service) taskStudioBatchRunOrDefault() *taskStudioBatchRunService {
	return s.resolveTaskStudioBatchRunCollaborators().batchRun
}

func (s *service) studioBatchRunExecutorOrDefault() *taskStudioBatchRunExecutor {
	return s.resolveTaskStudioBatchRunCollaborators().runExecutor
}

func (s *service) studioBatchRunCoordinatorOrDefault() *studioBatchRunCoordinator {
	return s.resolveTaskStudioBatchRunCollaborators().runCoordinator
}

func (s *service) resolveTaskStudioBatchRunCollaborators() taskStudioBatchRunCollaborators {
	if s == nil {
		return taskStudioBatchRunCollaborators{}
	}
	wiring := buildTaskStudioBatchRunCollaboratorWiring(s)
	s.studio.runGroup = wiring.resolve(s.studio.runGroup)
	return s.studio.runGroup
}
