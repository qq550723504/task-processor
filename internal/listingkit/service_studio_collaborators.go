package listingkit

func (s *service) taskStudioSessionOrDefault() *taskStudioSessionService {
	if s.taskStudioSession != nil {
		return s.taskStudioSession
	}
	s.taskStudioSession = newTaskStudioSessionService(buildTaskStudioSessionServiceConfig(s))
	return s.taskStudioSession
}

func (s *service) taskStudioBatchDraftOrDefault() *taskStudioBatchDraftService {
	if s.taskStudioBatchDraft != nil {
		return s.taskStudioBatchDraft
	}
	s.taskStudioBatchDraft = newTaskStudioBatchDraftService(buildTaskStudioBatchDraftServiceConfig(s))
	return s.taskStudioBatchDraft
}

func (s *service) taskStudioMediaOrDefault() *taskStudioMediaService {
	if s.taskStudioMedia != nil {
		return s.taskStudioMedia
	}
	s.taskStudioMedia = newTaskStudioMediaService(buildTaskStudioMediaServiceConfig(s))
	return s.taskStudioMedia
}

func (s *service) studioBatchGenerationOrDefault() *studioBatchGenerationService {
	if s.studioBatchGeneration != nil {
		return s.studioBatchGeneration
	}
	s.studioBatchGeneration = newStudioBatchGenerationService(buildStudioBatchGenerationServiceConfig(s))
	return s.studioBatchGeneration
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	if s.taskStudioBatch != nil {
		return s.taskStudioBatch
	}
	s.taskStudioBatch = newTaskStudioBatchService(buildTaskStudioBatchServiceConfig(s))
	return s.taskStudioBatch
}

func (s *service) taskStudioBatchRunOrDefault() *taskStudioBatchRunService {
	if s.taskStudioBatchRun != nil {
		return s.taskStudioBatchRun
	}
	s.taskStudioBatchRun = newTaskStudioBatchRunService(buildTaskStudioBatchRunServiceConfig(s))
	return s.taskStudioBatchRun
}
