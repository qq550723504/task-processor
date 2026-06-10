package httpapi

import (
	"context"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/shein/activity"
	"task-processor/internal/shein/api/marketing"
	"task-processor/internal/sheinlogin"
)

type listingKitSheinAPIClientFactory struct {
	repo        listingadmin.StoreRepository
	cookieStore *sheinlogin.RedisStore
}

func (f listingKitSheinAPIClientFactory) NewSheinAPIClient(storeID int64, storeInfo *listingkit.SheinStoreInfo) *listingkit.SheinRuntimeAPIClient {
	return listingkit.NewSheinRuntimeAPIClientWithStoreConfig(storeID, toSheinClientStoreConfig(storeInfo), boundSheinCookieProvider{
		store:    f.cookieStore,
		tenantID: tenantIDFromListingKitStoreInfo(storeInfo),
	})
}

type listingKitSheinRuntimeFactory struct {
	repo        listingadmin.StoreRepository
	cookieStore *sheinlogin.RedisStore
}

func (f listingKitSheinRuntimeFactory) NewAPIClient(ctx context.Context, storeID int64) *listingkit.SheinRuntimeAPIClient {
	storeInfo := f.resolveStoreConfig(ctx, storeID)
	return listingkit.NewSheinRuntimeAPIClientWithStoreConfig(storeID, storeInfo, boundSheinCookieProvider{
		store:    f.cookieStore,
		tenantID: tenantIDFromSheinClientStoreConfig(storeInfo),
	})
}

func (f listingKitSheinRuntimeFactory) resolveStoreConfig(ctx context.Context, storeID int64) *listingkit.SheinRuntimeStoreConfig {
	if f.repo == nil || storeID <= 0 {
		return nil
	}
	if tenantID := tenantIDFromContext(ctx); tenantID > 0 {
		store, err := f.repo.GetStore(ctx, tenantID, storeID)
		if err == nil && store != nil && store.ID > 0 {
			return toSheinClientStoreConfigFromListingAdmin(store)
		}
	}
	return nil
}

func buildListingKitSheinCategoryResolver(storeRepo listingadmin.StoreRepository, cookieStore *sheinlogin.RedisStore, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
	return sheinpub.NewCachedCategoryResolver(
		sheinpub.NewRuntimeCategoryResolver(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore}, llm),
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
	if apiClient == nil {
		return nil
	}
	baseClient := listingkit.NewSheinRuntimeBaseAPIClient(apiClient, apiClient.GetStoreID())
	return activity.NewActivityRegistrationService(nil, marketing.NewClient(baseClient))
}

type boundSheinCookieProvider struct {
	store    *sheinlogin.RedisStore
	tenantID int64
}

func (p boundSheinCookieProvider) GetCookie(ctx context.Context, storeID int64) (*listingkit.SheinRuntimeCookieLookupResult, error) {
	if p.store == nil || p.tenantID <= 0 || storeID <= 0 {
		return nil, nil
	}
	raw, ok, err := p.store.LoadCookieState(ctx, p.tenantID, storeID)
	if err != nil || !ok {
		return nil, err
	}
	cookieJSON, err := normalizeSheinCookiePayload(raw)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cookieJSON) == "" {
		return nil, nil
	}
	return &listingkit.SheinRuntimeCookieLookupResult{
		TenantID:   p.tenantID,
		CookieJSON: cookieJSON,
	}, nil
}
