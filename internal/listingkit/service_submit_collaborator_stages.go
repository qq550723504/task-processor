package listingkit

func (s *service) initializeSubmitTaskRecoveryCollaborators() {
	if s == nil {
		return
	}
	s.taskRecoveryOrDefault()
	s.taskRequeueOrDefault()
}

func (s *service) initializeSubmitStateCollaborators() {
	if s == nil {
		return
	}
	s.taskSubmissionExecutionOrDefault()
	s.taskSubmissionStateOrDefault()
}

func (s *service) initializeSubmitOrchestratorCollaborators() {
	if s == nil {
		return
	}
	s.taskSubmissionRecoveryOrDefault()
	s.taskDirectSubmissionOrDefault()
	s.taskSubmissionRefreshOrDefault()
	s.taskSubmissionOrDefault()
}

func (s *service) initializeSubmitWorkflowCollaborators() {
	if s == nil {
		return
	}
	s.taskTemporalSubmissionPersistenceOrDefault()
	s.taskTemporalSubmissionLifecycleOrDefault()
	s.taskTemporalSubmissionFlowOrDefault()
	s.taskTemporalSubmissionRefreshOrDefault()
}
