package httpapi

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/shein/activity"
	"task-processor/internal/shein/api/marketing"
	sheincategoryselector "task-processor/internal/shein/category"
	"task-processor/internal/sheinlogin"
)

func buildListingKitSheinCategoryResolver(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
	var aiConfig sheinpub.CategoryAIConfig
	if llm != nil {
		aiConfig.Selector = sheinCategorySelectorAdapter{selector: sheincategoryselector.NewOpenAISelector(llm)}
		aiConfig.SemanticVerifier = llm
	}
	return sheinpub.NewCachedCategoryResolver(
		sheinpub.NewRuntimeCategoryResolver(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore}, aiConfig),
		cache,
	)
}

type sheinCategorySelectorAdapter struct {
	selector sheincategoryselector.AISelector
}

func (a sheinCategorySelectorAdapter) SelectLevelOneCategoryByAI(ctx context.Context, title string, levelOneIDs []int, levelOneMap map[int]string) (int, error) {
	return a.selector.SelectLevelOneCategoryByAI(ctx, title, levelOneIDs, levelOneMap)
}

func (a sheinCategorySelectorAdapter) SelectCategoryByAI(ctx context.Context, title string, leafIDs []int, leafMap map[int]string) (int, error) {
	return a.selector.SelectCategoryByAI(ctx, title, leafIDs, leafMap)
}

func (a sheinCategorySelectorAdapter) ExtractCoreItemByAI(ctx context.Context, input sheinpub.CategoryCoreItemInput) (string, error) {
	return a.selector.ExtractCoreItemByAI(ctx, sheincategoryselector.CoreItemInput{
		Title:        input.Title,
		ProductType:  input.ProductType,
		CategoryPath: append([]string(nil), input.CategoryPath...),
		Attributes:   cloneSheinCategoryAttributes(input.Attributes),
	})
}

func cloneSheinCategoryAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	clone := make(map[string]string, len(attributes))
	for key, value := range attributes {
		clone[key] = value
	}
	return clone
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
	if apiClient == nil {
		return nil
	}
	baseClient := listingkit.NewSheinRuntimeBaseAPIClient(apiClient, apiClient.GetStoreID())
	return activity.NewActivityRegistrationService(nil, marketing.NewClient(baseClient))
}
