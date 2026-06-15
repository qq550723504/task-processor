package listingkit

func (s *service) initializeTaskReadCollaborators() {
	if s == nil {
		return
	}
	s.taskLifecycleOrDefault()
	s.taskGenerationOrDefault()
	s.taskRevisionOrDefault()
	s.taskPreviewOrDefault()
	s.taskExportOrDefault()
	s.sdsBaselineOrDefault()
}

func (s *service) initializeTaskStudioCollaborators() {
	if s == nil {
		return
	}
	s.initializeTaskStudioSessionCollaborators()
	s.initializeTaskStudioBatchCollaborators()
}

func (s *service) initializeTaskStudioSessionCollaborators() {
	if s == nil {
		return
	}
	s.taskStudioSessionOrDefault()
	s.taskStudioBatchDraftOrDefault()
	s.taskStudioMediaOrDefault()
}

func (s *service) initializeTaskStudioBatchCollaborators() {
	if s == nil {
		return
	}
	s.studioBatchGenerationOrDefault()
	s.taskStudioBatchOrDefault()
	s.taskStudioBatchRunOrDefault()
	s.studioBatchRunExecutorOrDefault()
	s.studioBatchRunCoordinatorOrDefault()
	s.initializeStudioBatchRunRecovery()
}
