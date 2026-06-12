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
	s.initializeTaskReadCollaborators()
	s.initializeTaskStudioCollaborators()
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
	s.initializeSettingsAdminCollaborators()
	s.initializeSheinAdminCollaborators()
}

func (s *service) initializeTemporalCollaborators() {
	if s == nil {
		return
	}
	s.submission.taskTemporalSubmissionAdapter = s.taskTemporalSubmissionAdapterOrDefault()
}
