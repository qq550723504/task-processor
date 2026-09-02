package listingkit

func (s *service) initializeTaskReadCollaborators() {
	if s == nil {
		return
	}
	s.taskLifecycleOrDefault()
	s.taskRevisionOrDefault()
	s.taskPreviewOrDefault()
	s.taskExportOrDefault()
	s.sdsBaselineOrDefault()
}
