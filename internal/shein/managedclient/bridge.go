package managedclient

import (
	"context"
	"strings"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	sheinclient "task-processor/internal/shein/client"
)

type runtimeBridge struct {
	client *management.ClientManager
}

func NewAPIClient(storeID int64, client *management.ClientManager) *sheinclient.APIClient {
	bridge := runtimeBridge{client: client}
	return sheinclient.NewAPIClientWithProviders(storeID, nil, bridge, bridge)
}

func NewAPIClientWithStoreInfo(storeID int64, client *management.ClientManager, storeInfo *managementapi.StoreRespDTO) *sheinclient.APIClient {
	bridge := runtimeBridge{client: client}
	return sheinclient.NewAPIClientWithProviders(storeID, storeConfigFromManagement(storeInfo), bridge, bridge)
}

func (b runtimeBridge) GetCookie(_ context.Context, storeID int64) (*sheinclient.CookieLookupResult, error) {
	if b.client == nil {
		return nil, nil
	}

	cookieJSON, tenantID, err := b.client.GetSheinCookie(storeID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cookieJSON) != "" {
		return &sheinclient.CookieLookupResult{
			TenantID:   tenantID,
			CookieJSON: cookieJSON,
		}, nil
	}

	storeClient := b.client.GetStoreClient()
	if storeClient == nil {
		return nil, nil
	}
	cookieJSON, err = storeClient.GetStoreCookie(storeID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cookieJSON) == "" {
		return nil, nil
	}

	if tenantID <= 0 {
		if storeInfo, lookupErr := storeClient.GetStore(storeID); lookupErr == nil && storeInfo != nil {
			tenantID = storeInfo.TenantID
		}
	}

	return &sheinclient.CookieLookupResult{
		TenantID:   tenantID,
		CookieJSON: cookieJSON,
	}, nil
}

func (b runtimeBridge) GetStoreConfig(_ context.Context, storeID int64) (*sheinclient.StoreConfig, error) {
	if b.client == nil {
		return nil, nil
	}
	storeClient := b.client.GetStoreClient()
	if storeClient == nil {
		return nil, nil
	}
	storeInfo, err := storeClient.GetStore(storeID)
	if err != nil {
		return nil, err
	}
	return storeConfigFromManagement(storeInfo), nil
}

func storeConfigFromManagement(storeInfo *managementapi.StoreRespDTO) *sheinclient.StoreConfig {
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
		LoginURL: strings.TrimSpace(storeInfo.LoginUrl),
		Proxy:    strings.TrimSpace(storeInfo.Proxy),
	}
}
