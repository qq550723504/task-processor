package listingkit

func (s *service) settingsAdminOrDefault() *settingsAdminService {
	return groupedCollaboratorOrBuild(&s.admin.settings, func() *settingsAdminService {
		return newSettingsAdminService(buildSettingsAdminServiceConfigWithWiring(buildSettingsAdminWiring(s)))
	})
}

func (s *service) sheinAdminOrDefault() *sheinAdminService {
	return groupedCollaboratorOrBuild(&s.admin.shein, func() *sheinAdminService {
		return newSheinAdminService(buildSheinAdminServiceConfigWithWiring(buildSheinAdminWiring(s)))
	})
}
