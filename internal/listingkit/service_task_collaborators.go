package listingkit

func (s *service) taskGenerationOrDefault() *taskGenerationService {
	if s.taskGeneration != nil {
		return s.taskGeneration
	}
	s.taskGeneration = newTaskGenerationService(buildTaskGenerationServiceConfig(s))
	return s.taskGeneration
}

func (s *service) taskRevisionOrDefault() *taskRevisionService {
	if s.taskRevision != nil {
		return s.taskRevision
	}
	s.taskRevision = newTaskRevisionService(buildTaskRevisionServiceConfig(s))
	return s.taskRevision
}

func (s *service) taskPreviewOrDefault() *taskPreviewService {
	if s.taskPreview != nil {
		return s.taskPreview
	}
	s.taskPreview = newTaskPreviewService(buildTaskPreviewServiceConfig(s))
	return s.taskPreview
}

func (s *service) taskLifecycleOrDefault() *taskLifecycleService {
	if s.taskLifecycle != nil {
		return s.taskLifecycle
	}
	s.taskLifecycle = newTaskLifecycleService(buildTaskLifecycleServiceConfig(s))
	return s.taskLifecycle
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	if s.sdsBaseline != nil {
		return s.sdsBaseline
	}
	s.sdsBaseline = newSDSBaselineService(buildSDSBaselineServiceConfig(s))
	return s.sdsBaseline
}
