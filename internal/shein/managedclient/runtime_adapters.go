package managedclient

import (
	"context"
	"strings"

	"task-processor/internal/listingruntime"
	sheinclient "task-processor/internal/shein/client"
)

type sheinCookieLookupSource interface {
	GetSheinCookie(storeID int64) (string, int64, error)
	GetSheinStoreCookie(storeID int64) (string, error)
}

type runtimeCookieProvider struct {
	source       sheinCookieLookupSource
	storeService listingruntime.StoreService
}

func NewRuntimeCookieProvider(source sheinCookieLookupSource, storeService listingruntime.StoreService) sheinclient.CookieProvider {
	if source == nil {
		return nil
	}
	return runtimeCookieProvider{
		source:       source,
		storeService: storeService,
	}
}

func (p runtimeCookieProvider) GetCookie(_ context.Context, storeID int64) (*sheinclient.CookieLookupResult, error) {
	cookieJSON, tenantID, err := p.source.GetSheinCookie(storeID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cookieJSON) == "" {
		cookieJSON, err = p.source.GetSheinStoreCookie(storeID)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(cookieJSON) == "" {
		return nil, nil
	}
	if tenantID <= 0 && p.storeService != nil {
		if storeInfo, lookupErr := p.storeService.GetStore(storeID); lookupErr == nil && storeInfo != nil {
			tenantID = storeInfo.TenantID
		}
	}
	return &sheinclient.CookieLookupResult{
		TenantID:   tenantID,
		CookieJSON: cookieJSON,
	}, nil
}

type runtimeStoreConfigProvider struct {
	storeService listingruntime.StoreService
}

func NewRuntimeStoreConfigProvider(storeService listingruntime.StoreService) sheinclient.StoreConfigProvider {
	if storeService == nil {
		return nil
	}
	return runtimeStoreConfigProvider{storeService: storeService}
}

func (p runtimeStoreConfigProvider) GetStoreConfig(_ context.Context, storeID int64) (*sheinclient.StoreConfig, error) {
	if p.storeService == nil {
		return nil, nil
	}
	storeInfo, err := p.storeService.GetStore(storeID)
	if err != nil {
		return nil, err
	}
	return storeConfigFromRuntime(storeInfo), nil
}
