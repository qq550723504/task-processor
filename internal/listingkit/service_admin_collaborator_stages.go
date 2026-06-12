package listingkit

func (s *service) initializeSettingsAdminCollaborators() {
	if s == nil {
		return
	}
	s.admin.settings = s.settingsAdminOrDefault()
}

func (s *service) initializeSheinAdminCollaborators() {
	if s == nil {
		return
	}
	s.admin.shein = s.sheinAdminOrDefault()
}
