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
	s.taskStudioSession = s.taskStudioSessionOrDefault()
	s.taskStudioBatchDraft = s.taskStudioBatchDraftOrDefault()
	s.taskStudioBatch = s.taskStudioBatchOrDefault()
	s.taskStudioBatchRun = s.taskStudioBatchRunOrDefault()
	s.taskStudioMedia = s.taskStudioMediaOrDefault()
	s.initializeStudioBatchRunRecovery()
}

func (s *service) initializeSubmitCollaborators() {
	if s == nil {
		return
	}

	// Task-level retry/recovery collaborators.
	s.submission.taskRecovery = s.taskRecoveryOrDefault()
	s.submission.taskRequeue = s.taskRequeueOrDefault()

	// Submission state and execution collaborators are initialized before
	// orchestrators that depend on them through config builders.
	s.submission.taskSubmissionState = s.taskSubmissionStateOrDefault()
	s.submission.taskSubmissionExecution = s.taskSubmissionExecutionOrDefault()

	// SHEIN submission orchestrators.
	s.submission.taskSubmissionRefresh = s.taskSubmissionRefreshOrDefault()
	s.submission.taskSubmissionRecovery = s.taskSubmissionRecoveryOrDefault()
	s.submission.taskDirectSubmission = s.taskDirectSubmissionOrDefault()
	s.submission.taskSubmission = s.taskSubmissionOrDefault()
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
