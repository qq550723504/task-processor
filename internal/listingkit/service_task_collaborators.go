package listingkit

func (s *service) taskGenerationOrDefault() *taskGenerationService {
	return groupedCollaboratorOrBuild(&s.task.generation, func() *taskGenerationService {
		return newTaskGenerationService(buildTaskGenerationServiceConfig(s))
	})
}

func (s *service) taskRevisionOrDefault() *taskRevisionService {
	return groupedCollaboratorOrBuild(&s.task.revision, func() *taskRevisionService {
		return newTaskRevisionService(buildTaskRevisionServiceConfig(s))
	})
}

func (s *service) taskPreviewOrDefault() taskPreviewReader {
	return groupedCollaboratorOrBuild(&s.task.preview, func() taskPreviewReader {
		return newTaskPreviewService(buildTaskPreviewServiceConfig(s))
	})
}

func (s *service) taskExportOrDefault() *taskExportService {
	return groupedCollaboratorOrBuild(&s.task.export, func() *taskExportService {
		return newTaskExportService(buildTaskExportServiceConfig(s))
	})
}

func (s *service) taskLifecycleOrDefault() *taskLifecycleService {
	return groupedCollaboratorOrBuild(&s.task.lifecycle, func() *taskLifecycleService {
		return newTaskLifecycleService(buildTaskLifecycleServiceConfig(s))
	})
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	return groupedCollaboratorOrBuild(&s.task.sdsBaseline, func() *sdsBaselineService {
		return newSDSBaselineService(buildSDSBaselineServiceConfig(s))
	})
}
