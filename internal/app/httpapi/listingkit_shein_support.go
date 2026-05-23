package httpapi

import (
	"context"
	"encoding/json"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
	sheinclient "task-processor/internal/shein/client"
	"task-processor/internal/sheinlogin"
	"task-processor/internal/tenantbridge"
)

type listingKitSheinAPIClientFactory struct {
	repo        listingadmin.StoreRepository
	cookieStore *sheinlogin.RedisStore
}

func (f listingKitSheinAPIClientFactory) NewSheinAPIClient(storeID int64, storeInfo *listingkit.SheinStoreInfo) *sheinclient.APIClient {
	return sheinclient.NewAPIClientWithStoreConfig(storeID, toSheinClientStoreConfig(storeInfo), boundSheinCookieProvider{
		store:    f.cookieStore,
		tenantID: tenantIDFromListingKitStoreInfo(storeInfo),
	})
}

type listingKitSheinRuntimeFactory struct {
	repo        listingadmin.StoreRepository
	cookieStore *sheinlogin.RedisStore
}

func (f listingKitSheinRuntimeFactory) NewAPIClient(ctx context.Context, storeID int64) *sheinclient.APIClient {
	storeInfo := f.resolveStoreConfig(ctx, storeID)
	return sheinclient.NewAPIClientWithStoreConfig(storeID, storeInfo, boundSheinCookieProvider{
		store:    f.cookieStore,
		tenantID: tenantIDFromSheinClientStoreConfig(storeInfo),
	})
}

func (f listingKitSheinRuntimeFactory) resolveStoreConfig(ctx context.Context, storeID int64) *sheinclient.StoreConfig {
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
		sheinpub.NewRuntimeSaleAttributeResolver(listingKitSheinRuntimeFactory{repo: storeRepo, cookieStore: cookieStore}, llm),
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

type boundSheinCookieProvider struct {
	store    *sheinlogin.RedisStore
	tenantID int64
}

func (p boundSheinCookieProvider) GetCookie(ctx context.Context, storeID int64) (*sheinclient.CookieLookupResult, error) {
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
	return &sheinclient.CookieLookupResult{
		TenantID:   p.tenantID,
		CookieJSON: cookieJSON,
	}, nil
}

func tenantIDFromContext(ctx context.Context) int64 {
	identity := openaiclient.IdentityFromContext(ctx)
	if identity.TenantID == "" {
		return 0
	}
	tenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, strings.TrimSpace(identity.TenantID))
	if err != nil || tenantID <= 0 {
		return 0
	}
	return tenantID
}

func tenantIDFromListingKitStoreInfo(storeInfo *listingkit.SheinStoreInfo) int64 {
	if storeInfo == nil {
		return 0
	}
	return storeInfo.TenantID
}

func tenantIDFromSheinClientStoreConfig(storeInfo *sheinclient.StoreConfig) int64 {
	if storeInfo == nil {
		return 0
	}
	return storeInfo.TenantID
}

func toSheinClientStoreConfig(storeInfo *listingkit.SheinStoreInfo) *sheinclient.StoreConfig {
	if storeInfo == nil {
		return nil
	}
	return &sheinclient.StoreConfig{
		ID:       storeInfo.ID,
		TenantID: storeInfo.TenantID,
		StoreID:  strings.TrimSpace(storeInfo.StoreID),
		Name:     strings.TrimSpace(storeInfo.Name),
		Platform: strings.TrimSpace(storeInfo.Platform),
		Region:   strings.TrimSpace(storeInfo.Region),
		LoginURL: strings.TrimSpace(storeInfo.LoginURL),
		Proxy:    strings.TrimSpace(storeInfo.Proxy),
	}
}

func toSheinClientStoreConfigFromListingAdmin(store *listingadmin.Store) *sheinclient.StoreConfig {
	if store == nil {
		return nil
	}
	return &sheinclient.StoreConfig{
		ID:       store.ID,
		TenantID: store.TenantID,
		StoreID:  strings.TrimSpace(store.StoreID),
		Name:     strings.TrimSpace(store.Name),
		Platform: strings.TrimSpace(store.Platform),
		Region:   strings.TrimSpace(store.Region),
		LoginURL: strings.TrimSpace(store.LoginURL),
		Proxy:    strings.TrimSpace(store.Proxy),
	}
}

func normalizeSheinCookiePayload(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil {
		if cookies, ok := wrapper["cookies"]; ok && len(cookies) > 0 && string(cookies) != "null" {
			return string(cookies), nil
		}
	}

	var list []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		return trimmed, nil
	}

	return trimmed, nil
}
