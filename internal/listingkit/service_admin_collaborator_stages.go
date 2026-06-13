package listingkit

func (s *service) initializeSettingsAdminCollaborators() {
	if s == nil {
		return
	}
	s.settingsAdminOrDefault()
}

func (s *service) initializeSheinAdminCollaborators() {
	if s == nil {
		return
	}
	s.sheinAdminOrDefault()
}
