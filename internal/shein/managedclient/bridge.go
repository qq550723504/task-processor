package managedclient

import (
	"context"
	"strings"

	"task-processor/internal/listingruntime"
	sheinclient "task-processor/internal/shein/client"
)

type runtimeBridge struct {
	cookieProvider sheinclient.CookieProvider
	storeProvider  sheinclient.StoreConfigProvider
}

func NewAPIClient(storeID int64, cookieProvider sheinclient.CookieProvider, storeProvider sheinclient.StoreConfigProvider) *sheinclient.APIClient {
	bridge := runtimeBridge{
		cookieProvider: cookieProvider,
		storeProvider:  storeProvider,
	}
	return sheinclient.NewAPIClientWithProviders(storeID, nil, bridge, bridge)
}

func NewAPIClientWithStoreInfo(storeID int64, cookieProvider sheinclient.CookieProvider, storeProvider sheinclient.StoreConfigProvider, storeInfo *listingruntime.StoreInfo) *sheinclient.APIClient {
	bridge := runtimeBridge{
		cookieProvider: cookieProvider,
		storeProvider:  storeProvider,
	}
	return sheinclient.NewAPIClientWithProviders(storeID, storeConfigFromRuntime(storeInfo), bridge, bridge)
}

func (b runtimeBridge) GetCookie(_ context.Context, storeID int64) (*sheinclient.CookieLookupResult, error) {
	if b.cookieProvider == nil {
		return nil, nil
	}
	result, err := b.cookieProvider.GetCookie(context.Background(), storeID)
	if err != nil {
		return nil, err
	}
	if result == nil || strings.TrimSpace(result.CookieJSON) == "" {
		return nil, nil
	}
	return result, nil
}

func (b runtimeBridge) GetStoreConfig(_ context.Context, storeID int64) (*sheinclient.StoreConfig, error) {
	if b.storeProvider == nil {
		return nil, nil
	}
	storeInfo, err := b.storeProvider.GetStoreConfig(context.Background(), storeID)
	if err != nil {
		return nil, err
	}
	return storeInfo, nil
}

func storeConfigFromRuntime(storeInfo *listingruntime.StoreInfo) *sheinclient.StoreConfig {
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
