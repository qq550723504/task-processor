package httpapi

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingruntime"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/shein/activity"
	"task-processor/internal/shein/api/marketing"
	"task-processor/internal/sheinlogin"
)

func buildListingKitSheinCategoryResolver(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
	var aiConfig sheinpub.CategoryAIConfig
	if llm != nil {
		aiConfig.Selector = newSheinCategorySelectorAdapter(llm)
		aiConfig.SemanticVerifier = llm
	}
	return sheinpub.NewCachedCategoryResolver(
		sheinpub.NewRuntimeCategoryResolver(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore}, aiConfig),
		cache,
	)
}

func buildListingKitSheinAttributeResolver(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver {
	return sheinpub.NewCachedAttributeResolver(
		sheinpub.NewRuntimeAttributeResolver(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore}, llm),
		cache,
	)
}

func buildListingKitSheinSaleAttributeResolver(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver {
	return sheinpub.NewCachedSaleAttributeResolver(
		sheinpub.NewRuntimeSaleAttributeResolver(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore}, llm, cache),
		cache,
	)
}

func buildListingKitSheinProductAPIBuilder(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore) sheinpub.ProductAPIBuilder {
	return sheinpub.NewRuntimeProductAPIBuilder(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore})
}

func buildListingKitSheinImageAPIBuilder(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore) sheinpub.ImageAPIBuilder {
	return sheinpub.NewRuntimeImageAPIBuilder(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore})
}

func buildListingKitSheinTranslateAPIBuilder(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore) sheinpub.TranslateAPIBuilder {
	return sheinpub.NewRuntimeTranslateAPIBuilder(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore})
}

func buildListingKitPromotionRegistrationBridge(apiClient *listingkit.SheinRuntimeAPIClient) activity.PromotionRegistrationBridge {
	return buildListingKitPromotionRegistrationBridgeWithDependencies(apiClient, nil, nil, nil)
}

func buildListingKitPromotionRegistrationBridgeWithDependencies(
	apiClient *listingkit.SheinRuntimeAPIClient,
	storeService listingruntime.StoreService,
	mappingRepo listingadmin.ProductImportMappingRepository,
	productDataRepo listingadmin.ProductDataRepository,
) activity.PromotionRegistrationBridge {
	if apiClient == nil {
		return nil
	}
	baseClient := listingkit.NewSheinRuntimeBaseAPIClient(apiClient, apiClient.GetStoreID())
	return activity.NewActivityRegistrationService(
		storeService,
		nil,
		listingKitActivityMappingFinder{repo: mappingRepo},
		productDataRepo,
		marketing.NewClient(baseClient),
	)
}

type listingKitActivityMappingFinder struct {
	repo listingadmin.ProductImportMappingRepository
}

func (f listingKitActivityMappingFinder) FindLatest(ctx context.Context, query listingadmin.ProductImportMappingQuery) (*listingadmin.ProductImportMapping, error) {
	if f.repo == nil {
		return nil, nil
	}
	page, err := f.repo.ListProductImportMappings(ctx, query)
	if err != nil || page == nil || len(page.Items) == 0 {
		return nil, err
	}
	mapping := page.Items[0]
	return &mapping, nil
}
