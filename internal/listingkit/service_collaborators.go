package listingkit

func (s *service) initializeCollaborators() {
	s.runCollaboratorInitializers(
		s.initializeTaskCollaborators,
		s.initializeAdminCollaborators,
		s.initializeSubmitCollaborators,
	)
}

func (s *service) initializeTaskCollaborators() {
	s.runCollaboratorInitializers(
		s.initializeTaskReadCollaborators,
		s.initializeTaskStudioCollaborators,
	)
}

func (s *service) initializeSubmitCollaborators() {
	s.runCollaboratorInitializers(
		s.initializeSubmitTaskRecoveryCollaborators,
		s.initializeSubmitStateCollaborators,
		s.initializeSubmitOrchestratorCollaborators,
		s.initializeSubmitWorkflowCollaborators,
	)
}

func (s *service) initializeAdminCollaborators() {
	s.runCollaboratorInitializers(
		s.initializeSettingsAdminCollaborators,
		s.initializeSheinAdminCollaborators,
	)
}
