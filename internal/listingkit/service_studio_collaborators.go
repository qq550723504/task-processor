package listingkit

func (s *service) taskStudioSessionOrDefault() *taskStudioSessionService {
	s.ensureTaskStudioSessionCollaborators()
	return s.studio.session
}

func (s *service) taskStudioBatchDraftOrDefault() *taskStudioBatchDraftService {
	s.ensureTaskStudioSessionCollaborators()
	return s.studio.batchDraft
}

func (s *service) taskStudioMediaOrDefault() *taskStudioMediaService {
	s.ensureTaskStudioSessionCollaborators()
	return s.studio.media
}

func (s *service) ensureTaskStudioSessionCollaborators() {
	if s == nil {
		return
	}
	wiring := buildTaskStudioSessionCollaboratorWiring(s)
	collaborators := wiring.resolve(s.studio)
	s.studio.session = collaborators.session
	s.studio.batchDraft = collaborators.batchDraft
	s.studio.media = collaborators.media
}

func (s *service) studioBatchGenerationOrDefault() *studioBatchGenerationService {
	s.ensureTaskStudioBatchCollaborators()
	return s.studio.batchGeneration
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	s.ensureTaskStudioBatchCollaborators()
	return s.studio.batch
}

func (s *service) ensureTaskStudioBatchCollaborators() {
	if s == nil {
		return
	}
	wiring := buildTaskStudioBatchCollaboratorWiring(s)
	collaborators := wiring.resolve(s.studio)
	s.studio.batchGeneration = collaborators.batchGeneration
	s.studio.batch = collaborators.batch
}

func (s *service) taskStudioBatchRunOrDefault() *taskStudioBatchRunService {
	s.ensureTaskStudioBatchRunCollaborators()
	return s.studio.batchRun
}

func (s *service) studioBatchRunExecutorOrDefault() *taskStudioBatchRunExecutor {
	s.ensureTaskStudioBatchRunCollaborators()
	return s.studio.runExecutor
}

func (s *service) studioBatchRunCoordinatorOrDefault() *studioBatchRunCoordinator {
	s.ensureTaskStudioBatchRunCollaborators()
	return s.studio.runCoordinator
}

func (s *service) ensureTaskStudioBatchRunCollaborators() {
	if s == nil {
		return
	}
	wiring := buildTaskStudioBatchRunCollaboratorWiring(s)
	collaborators := wiring.resolve(s.studio)
	s.studio.batchRun = collaborators.batchRun
	s.studio.runExecutor = collaborators.runExecutor
	s.studio.runCoordinator = collaborators.runCoordinator
}
