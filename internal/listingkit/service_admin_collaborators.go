package listingkit

func (s *service) settingsAdminOrDefault() *settingsAdminService {
	if s.admin.settings != nil {
		s.settingsAdmin = s.admin.settings
		return s.admin.settings
	}
	if s.settingsAdmin != nil {
		s.admin.settings = s.settingsAdmin
		return s.settingsAdmin
	}
	service := newSettingsAdminService(buildSettingsAdminServiceConfig(s))
	s.admin.settings = service
	s.settingsAdmin = service
	return service
}

func (s *service) sheinAdminOrDefault() *sheinAdminService {
	if s.admin.shein != nil {
		s.sheinAdmin = s.admin.shein
		return s.admin.shein
	}
	if s.sheinAdmin != nil {
		s.admin.shein = s.sheinAdmin
		return s.sheinAdmin
	}
	service := newSheinAdminService(buildSheinAdminServiceConfig(s))
	s.admin.shein = service
	s.sheinAdmin = service
	return service
}
