package listingkit

func (s *service) initializeSettingsAdminCollaborators() {
	if s == nil {
		return
	}
	s.settingsAdmin = s.settingsAdminOrDefault()
}

func (s *service) initializeSheinAdminCollaborators() {
	if s == nil {
		return
	}
	s.sheinAdmin = s.sheinAdminOrDefault()
}
