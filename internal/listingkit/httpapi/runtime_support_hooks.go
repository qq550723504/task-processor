package httpapi

import (
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/sheinlogin"
)

func buildRuntimeSupportHooks(cookieStore *sheinlogin.RedisStore) BuildServiceHooks {
	return BuildServiceHooks{
		SheinPricingPolicyBuilder:        BuildSheinPricingPolicy,
		ImageUploadStoreBuilder:          BuildImageUploadStore,
		LegacyTenantResolverConfigurator: ConfigureLegacyTenantResolver,
		SheinCategoryLLMClientBuilder:    BuildSheinCategoryLLMClient,
		SheinSaleAttributeLLMBuilder:     BuildSheinSaleAttributeLLMClient,
		SheinCategoryResolverBuilder: func(storeRepo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
			return buildListingKitSheinCategoryResolver(storeRepo, cookieStore, llm, cache)
		},
		SheinAttributeResolverBuilder: func(storeRepo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver {
			return buildListingKitSheinAttributeResolver(storeRepo, cookieStore, llm, cache)
		},
		SheinSaleAttributeResolverBuilder: func(storeRepo listingadmin.StoreRepository, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver {
			return buildListingKitSheinSaleAttributeResolver(storeRepo, cookieStore, llm, cache)
		},
		SheinProductAPIBuilderFactory: func(storeRepo listingadmin.StoreRepository) sheinpub.ProductAPIBuilder {
			return buildListingKitSheinProductAPIBuilder(storeRepo, cookieStore)
		},
		SheinImageAPIBuilderFactory: func(storeRepo listingadmin.StoreRepository) sheinpub.ImageAPIBuilder {
			return buildListingKitSheinImageAPIBuilder(storeRepo, cookieStore)
		},
		SheinTranslateAPIBuilderFactory: func(storeRepo listingadmin.StoreRepository) sheinpub.TranslateAPIBuilder {
			return buildListingKitSheinTranslateAPIBuilder(storeRepo, cookieStore)
		},
		SheinAPIClientFactoryBuilder: func(storeRepo listingadmin.StoreRepository) listingkit.SheinAPIClientFactory {
			return listingKitSheinAPIClientFactory{repo: storeRepo, cookieStore: cookieStore}
		},
		StudioImageGeneratorBuilder: BuildStudioImageGenerator,
		DefaultSheinStoreIDResolver: listingkit.ResolveDefaultSheinStoreID,
		ConfigureZitadelAuth:        ConfigureListingKitZitadelAuth,
		ConfigureAuthorization:      ConfigureListingKitAuthorization,
	}
}
