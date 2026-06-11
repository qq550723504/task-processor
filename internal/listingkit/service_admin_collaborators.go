package listingkit

func (s *service) settingsAdminOrDefault() *settingsAdminService {
	if s.settingsAdmin != nil {
		return s.settingsAdmin
	}
	s.settingsAdmin = newSettingsAdminService(buildSettingsAdminServiceConfig(s))
	return s.settingsAdmin
}

func (s *service) sheinAdminOrDefault() *sheinAdminService {
	if s.sheinAdmin != nil {
		return s.sheinAdmin
	}
	s.sheinAdmin = newSheinAdminService(buildSheinAdminServiceConfig(s))
	return s.sheinAdmin
}
