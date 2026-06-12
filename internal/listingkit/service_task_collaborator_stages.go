package listingkit

func (s *service) initializeTaskReadCollaborators() {
	if s == nil {
		return
	}
	s.taskLifecycle = s.taskLifecycleOrDefault()
	s.taskGeneration = s.taskGenerationOrDefault()
	s.taskRevision = s.taskRevisionOrDefault()
	s.taskPreview = s.taskPreviewOrDefault()
	s.taskExport = s.taskExportOrDefault()
	s.sdsBaseline = s.sdsBaselineOrDefault()
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
	s.taskStudioSession = s.taskStudioSessionOrDefault()
	s.taskStudioBatchDraft = s.taskStudioBatchDraftOrDefault()
	s.taskStudioMedia = s.taskStudioMediaOrDefault()
}

func (s *service) initializeTaskStudioBatchCollaborators() {
	if s == nil {
		return
	}
	s.studioBatchGeneration = s.studioBatchGenerationOrDefault()
	s.taskStudioBatch = s.taskStudioBatchOrDefault()
	s.taskStudioBatchRun = s.taskStudioBatchRunOrDefault()
	s.initializeStudioBatchRunRecovery()
}
