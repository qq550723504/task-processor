package listingkit

func (s *service) initializeTaskReadCollaborators() {
	if s == nil {
		return
	}
	s.task.lifecycle = s.taskLifecycleOrDefault()
	s.task.generation = s.taskGenerationOrDefault()
	s.task.revision = s.taskRevisionOrDefault()
	s.task.preview = s.taskPreviewOrDefault()
	s.task.export = s.taskExportOrDefault()
	s.task.sdsBaseline = s.sdsBaselineOrDefault()
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
	s.studio.session = s.taskStudioSessionOrDefault()
	s.studio.batchDraft = s.taskStudioBatchDraftOrDefault()
	s.studio.media = s.taskStudioMediaOrDefault()
}

func (s *service) initializeTaskStudioBatchCollaborators() {
	if s == nil {
		return
	}
	s.studio.batchGeneration = s.studioBatchGenerationOrDefault()
	s.studio.batch = s.taskStudioBatchOrDefault()
	s.studio.batchRun = s.taskStudioBatchRunOrDefault()
	s.initializeStudioBatchRunRecovery()
}
