package listingkit

import "context"

type settingsAdminWiring struct {
	storeProfileRepo     StoreProfileRepository
	aiCredentialStore    AIClientCredentialStore
	currentSheinSettings func() SheinSettings
	mutateSheinSettings  func(func(*SheinSettings)) SheinSettings
	listStoreOptions     func(context.Context) []SheinStoreOption
}

func buildSettingsAdminWiring(s *service) settingsAdminWiring {
	return settingsAdminWiring{
		storeProfileRepo:     s.storeProfileRepo,
		aiCredentialStore:    s.aiCredentialStore,
		currentSheinSettings: s.currentSheinSubmitSettings,
		mutateSheinSettings: func(mutate func(*SheinSettings)) SheinSettings {
			s.sheinSettingsMu.Lock()
			defer s.sheinSettingsMu.Unlock()
			settings := s.sheinSettings
			if mutate != nil {
				mutate(&settings)
			}
			s.sheinSettings = settings
			return settings
		},
		listStoreOptions: s.listSheinStoreOptions,
	}
}
