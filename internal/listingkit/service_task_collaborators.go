package listingkit

func (s *service) taskGenerationOrDefault() *taskGenerationService {
	if s.task.generation != nil {
		s.taskGeneration = s.task.generation
		return s.task.generation
	}
	if s.taskGeneration != nil {
		s.task.generation = s.taskGeneration
		return s.taskGeneration
	}
	service := newTaskGenerationService(buildTaskGenerationServiceConfig(s))
	s.task.generation = service
	s.taskGeneration = service
	return service
}

func (s *service) taskRevisionOrDefault() *taskRevisionService {
	if s.task.revision != nil {
		s.taskRevision = s.task.revision
		return s.task.revision
	}
	if s.taskRevision != nil {
		s.task.revision = s.taskRevision
		return s.taskRevision
	}
	service := newTaskRevisionService(buildTaskRevisionServiceConfig(s))
	s.task.revision = service
	s.taskRevision = service
	return service
}

func (s *service) taskPreviewOrDefault() *taskPreviewService {
	if s.task.preview != nil {
		s.taskPreview = s.task.preview
		return s.task.preview
	}
	if s.taskPreview != nil {
		s.task.preview = s.taskPreview
		return s.taskPreview
	}
	service := newTaskPreviewService(buildTaskPreviewServiceConfig(s))
	s.task.preview = service
	s.taskPreview = service
	return service
}

func (s *service) taskExportOrDefault() *taskExportService {
	if s.task.export != nil {
		s.taskExport = s.task.export
		return s.task.export
	}
	if s.taskExport != nil {
		s.task.export = s.taskExport
		return s.taskExport
	}
	service := newTaskExportService(buildTaskExportServiceConfig(s))
	s.task.export = service
	s.taskExport = service
	return service
}

func (s *service) taskLifecycleOrDefault() *taskLifecycleService {
	if s.task.lifecycle != nil {
		s.taskLifecycle = s.task.lifecycle
		return s.task.lifecycle
	}
	if s.taskLifecycle != nil {
		s.task.lifecycle = s.taskLifecycle
		return s.taskLifecycle
	}
	service := newTaskLifecycleService(buildTaskLifecycleServiceConfig(s))
	s.task.lifecycle = service
	s.taskLifecycle = service
	return service
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	if s.task.sdsBaseline != nil {
		s.sdsBaseline = s.task.sdsBaseline
		return s.task.sdsBaseline
	}
	if s.sdsBaseline != nil {
		s.task.sdsBaseline = s.sdsBaseline
		return s.sdsBaseline
	}
	service := newSDSBaselineService(buildSDSBaselineServiceConfig(s))
	s.task.sdsBaseline = service
	s.sdsBaseline = service
	return service
}
