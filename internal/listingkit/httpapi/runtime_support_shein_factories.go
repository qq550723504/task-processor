package httpapi

import (
	"context"
	"strings"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
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
