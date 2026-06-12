package listingkit

func (s *service) initializeCollaborators() {
	if s == nil {
		return
	}
	s.initializeTaskCollaborators()
	s.initializeAdminCollaborators()
	s.initializeSubmitCollaborators()
	s.initializeTemporalCollaborators()
}

func (s *service) initializeTaskCollaborators() {
	if s == nil {
		return
	}
	s.taskLifecycle = s.taskLifecycleOrDefault()
	s.taskGeneration = s.taskGenerationOrDefault()
	s.taskRevision = s.taskRevisionOrDefault()
	s.taskPreview = s.taskPreviewOrDefault()
	s.sdsBaseline = s.sdsBaselineOrDefault()
	s.taskStudioSession = s.taskStudioSessionOrDefault()
	s.taskStudioBatchDraft = s.taskStudioBatchDraftOrDefault()
	s.studioBatchGeneration = s.studioBatchGenerationOrDefault()
	s.taskStudioBatch = s.taskStudioBatchOrDefault()
	s.taskStudioBatchRun = s.taskStudioBatchRunOrDefault()
	s.taskStudioMedia = s.taskStudioMediaOrDefault()
	s.initializeStudioBatchRunRecovery()
}

func (s *service) initializeSubmitCollaborators() {
	if s == nil {
		return
	}

	s.initializeSubmitTaskRecoveryCollaborators()
	s.initializeSubmitStateCollaborators()
	s.initializeSubmitOrchestratorCollaborators()
}

func (s *service) initializeAdminCollaborators() {
	if s == nil {
		return
	}
	s.settingsAdmin = s.settingsAdminOrDefault()
	s.sheinAdmin = s.sheinAdminOrDefault()
}

func (s *service) initializeTemporalCollaborators() {
	if s == nil {
		return
	}
	s.submission.taskTemporalSubmissionAdapter = s.taskTemporalSubmissionAdapterOrDefault()
}
