package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinclient "task-processor/internal/shein/client"
)

type settingsAdminWiring struct {
	storeProfileRepo     StoreProfileRepository
	aiCredentialStore    AIClientCredentialStore
	currentSheinSettings func() SheinSettings
	mutateSheinSettings  func(func(*SheinSettings)) SheinSettings
	listStoreOptions     func(context.Context) []SheinStoreOption
}

type sheinAdminWiring struct {
	repo                  Repository
	mutateTaskResult      func(context.Context, string, TaskResultMutation) (*Task, error)
	currentPricingRule    func() sheinpub.PricingRule
	newSheinAPIClient     func(context.Context, *Task) (*sheinclient.APIClient, int64, error)
	buildTaskPreview      func(context.Context, *Task, string) (*ListingKitPreview, error)
	categoryResolver      sheinpub.CategoryResolver
	attributeResolver     sheinpub.AttributeResolver
	saleAttributeResolver sheinpub.SaleAttributeResolver
	clearPricingCache     func(*sheinpub.BuildRequest, *sheinpub.Package) error
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

func buildSheinAdminWiring(s *service) sheinAdminWiring {
	return sheinAdminWiring{
		repo:                  s.repo,
		mutateTaskResult:      s.mutateTaskResult,
		currentPricingRule:    s.currentSheinPricingRule,
		newSheinAPIClient:     s.newSheinAPIClient,
		buildTaskPreview:      s.buildTaskPreview,
		categoryResolver:      s.sheinCategoryResolver,
		attributeResolver:     s.sheinAttributeResolver,
		saleAttributeResolver: s.sheinSaleAttributeResolver,
		clearPricingCache:     s.clearSheinPricingCache,
	}
}
