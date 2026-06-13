package listingkit

func (s *service) taskGenerationOrDefault() *taskGenerationService {
	return syncGroupedCollaborator(&s.task.generation, &s.collabMirrors.taskGeneration, func() *taskGenerationService {
		return newTaskGenerationService(buildTaskGenerationServiceConfig(s))
	})
}

func (s *service) taskRevisionOrDefault() *taskRevisionService {
	return syncGroupedCollaborator(&s.task.revision, &s.collabMirrors.taskRevision, func() *taskRevisionService {
		return newTaskRevisionService(buildTaskRevisionServiceConfig(s))
	})
}

func (s *service) taskPreviewOrDefault() *taskPreviewService {
	return syncGroupedCollaborator(&s.task.preview, &s.collabMirrors.taskPreview, func() *taskPreviewService {
		return newTaskPreviewService(buildTaskPreviewServiceConfig(s))
	})
}

func (s *service) taskExportOrDefault() *taskExportService {
	return syncGroupedCollaborator(&s.task.export, &s.collabMirrors.taskExport, func() *taskExportService {
		return newTaskExportService(buildTaskExportServiceConfig(s))
	})
}

func (s *service) taskLifecycleOrDefault() *taskLifecycleService {
	return syncGroupedCollaborator(&s.task.lifecycle, &s.collabMirrors.taskLifecycle, func() *taskLifecycleService {
		return newTaskLifecycleService(buildTaskLifecycleServiceConfig(s))
	})
}

func (s *service) sdsBaselineOrDefault() *sdsBaselineService {
	return syncGroupedCollaborator(&s.task.sdsBaseline, &s.collabMirrors.sdsBaseline, func() *sdsBaselineService {
		return newSDSBaselineService(buildSDSBaselineServiceConfig(s))
	})
}
