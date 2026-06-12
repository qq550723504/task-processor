package listingkit

func (s *service) initializeCollaborators() {
	if s == nil {
		return
	}
	s.initializeTaskCollaborators()
	s.initializeAdminCollaborators()
	s.initializeSubmitCollaborators()
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
	s.initializeSubmitWorkflowCollaborators()
}

func (s *service) initializeAdminCollaborators() {
	if s == nil {
		return
	}
	s.initializeSettingsAdminCollaborators()
	s.initializeSheinAdminCollaborators()
}
