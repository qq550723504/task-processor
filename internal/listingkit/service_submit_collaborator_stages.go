package listingkit

func (s *service) initializeSubmitTaskRecoveryCollaborators() {
	if s == nil {
		return
	}
	s.ensureTaskSubmitTaskRecoveryCollaborators()
}

func (s *service) initializeSubmitStateCollaborators() {
	if s == nil {
		return
	}
	s.ensureTaskSubmissionCoreCollaborators()
}

func (s *service) initializeSubmitOrchestratorCollaborators() {
	if s == nil {
		return
	}
	s.ensureTaskManagedSubmissionCollaborators()
}

func (s *service) initializeSubmitWorkflowCollaborators() {
	if s == nil {
		return
	}
	s.ensureTaskTemporalSubmissionCollaborators()
}
