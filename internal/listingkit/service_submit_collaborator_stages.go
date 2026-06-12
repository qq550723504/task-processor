package listingkit

func (s *service) initializeSubmitTaskRecoveryCollaborators() {
	if s == nil {
		return
	}
	s.submission.taskRecovery = s.taskRecoveryOrDefault()
	s.submission.taskRequeue = s.taskRequeueOrDefault()
}

func (s *service) initializeSubmitStateCollaborators() {
	if s == nil {
		return
	}
	s.submission.taskSubmissionState = s.taskSubmissionStateOrDefault()
	s.submission.taskSubmissionExecution = s.taskSubmissionExecutionOrDefault()
}

func (s *service) initializeSubmitOrchestratorCollaborators() {
	if s == nil {
		return
	}
	s.submission.taskSubmissionRefresh = s.taskSubmissionRefreshOrDefault()
	s.submission.taskSubmissionRecovery = s.taskSubmissionRecoveryOrDefault()
	s.submission.taskDirectSubmission = s.taskDirectSubmissionOrDefault()
	s.submission.taskSubmission = s.taskSubmissionOrDefault()
}

func (s *service) initializeSubmitWorkflowCollaborators() {
	if s == nil {
		return
	}
	s.submission.taskTemporalSubmissionAdapter = s.taskTemporalSubmissionAdapterOrDefault()
}
