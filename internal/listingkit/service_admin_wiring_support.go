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
	recovery              *taskSubmissionRecoveryService
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
		storeProfileRepo:     resolveAdminStoreProfileRepo(s),
		aiCredentialStore:    resolveAdminAICredentialStore(s),
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
	repository := buildServiceRepositoryWiring(s)
	preview := buildTaskPreviewAccessWiring(s)
	return sheinAdminWiring{
		repo:                  repository.repo,
		recovery:              s.taskSubmissionRecoveryOrDefault(),
		currentPricingRule:    s.currentSheinPricingRule,
		newSheinAPIClient:     s.newSheinAPIClient,
		buildTaskPreview:      preview.buildTaskPreview,
		categoryResolver:      resolveSheinCategoryResolver(s),
		attributeResolver:     resolveSheinAttributeResolver(s),
		saleAttributeResolver: resolveSheinSaleAttributeResolver(s),
		clearPricingCache:     s.clearSheinPricingCache,
	}
}

func resolveAdminStoreProfileRepo(s *service) StoreProfileRepository {
	if s == nil {
		return nil
	}
	return s.adminDeps.storeProfileRepo
}

func resolveAdminAICredentialStore(s *service) AIClientCredentialStore {
	if s == nil {
		return nil
	}
	return s.adminDeps.aiCredentialStore
}

func buildSettingsAdminServiceConfigWithWiring(wiring settingsAdminWiring) settingsAdminServiceConfig {
	return settingsAdminServiceConfig{
		storeProfileRepo:     wiring.storeProfileRepo,
		aiCredentialStore:    wiring.aiCredentialStore,
		currentSheinSettings: wiring.currentSheinSettings,
		mutateSheinSettings:  wiring.mutateSheinSettings,
		listStoreOptions:     wiring.listStoreOptions,
	}
}

func buildSheinAdminServiceConfigWithWiring(wiring sheinAdminWiring) sheinAdminServiceConfig {
	return sheinAdminServiceConfig{
		repo:                  wiring.repo,
		recovery:              wiring.recovery,
		currentPricingRule:    wiring.currentPricingRule,
		newSheinAPIClient:     wiring.newSheinAPIClient,
		buildTaskPreview:      wiring.buildTaskPreview,
		categoryResolver:      wiring.categoryResolver,
		attributeResolver:     wiring.attributeResolver,
		saleAttributeResolver: wiring.saleAttributeResolver,
		clearPricingCache:     wiring.clearPricingCache,
	}
}
