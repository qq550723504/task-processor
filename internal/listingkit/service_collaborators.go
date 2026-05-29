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
	s.taskStudioMedia = s.taskStudioMediaOrDefault()
}

func (s *service) initializeSubmitCollaborators() {
	if s == nil {
		return
	}
	s.taskSubmissionRecovery = s.taskSubmissionRecoveryOrDefault()
	s.taskSubmission = s.taskSubmissionOrDefault()
	s.taskSubmissionExecution = s.taskSubmissionExecutionOrDefault()
	s.taskSubmissionState = s.taskSubmissionStateOrDefault()
	s.taskDirectSubmission = s.taskDirectSubmissionOrDefault()
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
	s.taskTemporalSubmissionAdapter = s.taskTemporalSubmissionAdapterOrDefault()
}
