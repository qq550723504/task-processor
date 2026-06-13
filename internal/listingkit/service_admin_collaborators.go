package listingkit

func (s *service) settingsAdminOrDefault() *settingsAdminService {
	return syncGroupedCollaborator(&s.admin.settings, &s.collabMirrors.settingsAdmin, func() *settingsAdminService {
		return newSettingsAdminService(buildSettingsAdminServiceConfig(s))
	})
}

func (s *service) sheinAdminOrDefault() *sheinAdminService {
	return syncGroupedCollaborator(&s.admin.shein, &s.collabMirrors.sheinAdmin, func() *sheinAdminService {
		return newSheinAdminService(buildSheinAdminServiceConfig(s))
	})
}
