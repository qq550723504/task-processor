package listingkit

func (s *service) taskStudioSessionOrDefault() *taskStudioSessionService {
	if s.studio.session != nil {
		s.taskStudioSession = s.studio.session
		return s.studio.session
	}
	if s.taskStudioSession != nil {
		s.studio.session = s.taskStudioSession
		return s.taskStudioSession
	}
	service := newTaskStudioSessionService(buildTaskStudioSessionServiceConfig(s))
	s.studio.session = service
	s.taskStudioSession = service
	return service
}

func (s *service) taskStudioBatchDraftOrDefault() *taskStudioBatchDraftService {
	if s.studio.batchDraft != nil {
		s.taskStudioBatchDraft = s.studio.batchDraft
		return s.studio.batchDraft
	}
	if s.taskStudioBatchDraft != nil {
		s.studio.batchDraft = s.taskStudioBatchDraft
		return s.taskStudioBatchDraft
	}
	service := newTaskStudioBatchDraftService(buildTaskStudioBatchDraftServiceConfig(s))
	s.studio.batchDraft = service
	s.taskStudioBatchDraft = service
	return service
}

func (s *service) taskStudioMediaOrDefault() *taskStudioMediaService {
	if s.studio.media != nil {
		s.taskStudioMedia = s.studio.media
		return s.studio.media
	}
	if s.taskStudioMedia != nil {
		s.studio.media = s.taskStudioMedia
		return s.taskStudioMedia
	}
	service := newTaskStudioMediaService(buildTaskStudioMediaServiceConfig(s))
	s.studio.media = service
	s.taskStudioMedia = service
	return service
}

func (s *service) studioBatchGenerationOrDefault() *studioBatchGenerationService {
	if s.studio.batchGeneration != nil {
		s.studioBatchGeneration = s.studio.batchGeneration
		return s.studio.batchGeneration
	}
	if s.studioBatchGeneration != nil {
		s.studio.batchGeneration = s.studioBatchGeneration
		return s.studioBatchGeneration
	}
	service := newStudioBatchGenerationService(buildStudioBatchGenerationServiceConfig(s))
	s.studio.batchGeneration = service
	s.studioBatchGeneration = service
	return service
}

func (s *service) taskStudioBatchOrDefault() *taskStudioBatchService {
	if s.studio.batch != nil {
		s.taskStudioBatch = s.studio.batch
		return s.studio.batch
	}
	if s.taskStudioBatch != nil {
		s.studio.batch = s.taskStudioBatch
		return s.taskStudioBatch
	}
	service := newTaskStudioBatchService(buildTaskStudioBatchServiceConfig(s))
	s.studio.batch = service
	s.taskStudioBatch = service
	return service
}

func (s *service) taskStudioBatchRunOrDefault() *taskStudioBatchRunService {
	if s.studio.batchRun != nil {
		s.taskStudioBatchRun = s.studio.batchRun
		return s.studio.batchRun
	}
	if s.taskStudioBatchRun != nil {
		s.studio.batchRun = s.taskStudioBatchRun
		return s.taskStudioBatchRun
	}
	service := newTaskStudioBatchRunService(buildTaskStudioBatchRunServiceConfig(s))
	s.studio.batchRun = service
	s.taskStudioBatchRun = service
	return service
}
