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
	syncGroupedDependency(&s.studio.session, &s.collabMirrors.taskStudioSession)
	syncGroupedDependency(&s.studio.batchDraft, &s.collabMirrors.taskStudioBatchDraft)
	syncGroupedDependency(&s.studio.media, &s.collabMirrors.taskStudioMedia)
	wiring := buildTaskStudioSessionCollaboratorWiring(s)
	collaborators := wiring.resolve(s.studio)
	s.studio.session = collaborators.session
	s.collabMirrors.taskStudioSession = collaborators.session
	s.studio.batchDraft = collaborators.batchDraft
	s.collabMirrors.taskStudioBatchDraft = collaborators.batchDraft
	s.studio.media = collaborators.media
	s.collabMirrors.taskStudioMedia = collaborators.media
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
	syncGroupedDependency(&s.studio.batchGeneration, &s.collabMirrors.studioBatchGeneration)
	syncGroupedDependency(&s.studio.batch, &s.collabMirrors.taskStudioBatch)
	wiring := buildTaskStudioBatchCollaboratorWiring(s)
	collaborators := wiring.resolve(s.studio)
	s.studio.batchGeneration = collaborators.batchGeneration
	s.collabMirrors.studioBatchGeneration = collaborators.batchGeneration
	s.studio.batch = collaborators.batch
	s.collabMirrors.taskStudioBatch = collaborators.batch
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
	syncGroupedDependency(&s.studio.batchRun, &s.collabMirrors.taskStudioBatchRun)
	syncGroupedDependency(&s.studio.runExecutor, &s.collabMirrors.studioBatchRunExecutor)
	syncGroupedDependency(&s.studio.runCoordinator, &s.collabMirrors.studioBatchRunCoordinator)
	wiring := buildTaskStudioBatchRunCollaboratorWiring(s)
	collaborators := wiring.resolve(s.studio)
	s.studio.batchRun = collaborators.batchRun
	s.collabMirrors.taskStudioBatchRun = collaborators.batchRun
	s.studio.runExecutor = collaborators.runExecutor
	s.collabMirrors.studioBatchRunExecutor = collaborators.runExecutor
	s.studio.runCoordinator = collaborators.runCoordinator
	s.collabMirrors.studioBatchRunCoordinator = collaborators.runCoordinator
}
