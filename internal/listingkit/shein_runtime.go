package listingkit

import (
	"context"

	sheinclient "task-processor/internal/shein/client"
)

type SheinRuntimeAPIClient = sheinclient.APIClient
type SheinRuntimeStoreConfig = sheinclient.StoreConfig
type SheinRuntimeCookieLookupResult = sheinclient.CookieLookupResult
type SheinRuntimeCookieProvider = sheinclient.CookieProvider

type SheinStoreInfo struct {
	ID       int64
	TenantID int64
	StoreID  string
	Name     string
	Platform string
	Region   string
	LoginURL string
	Proxy    string
}

type SheinStoreCatalog interface {
	GetStoreInfo(ctx context.Context, tenantID, storeID int64) (*SheinStoreInfo, error)
	ListStoreOptions(ctx context.Context, tenantID int64) ([]SheinStoreOption, error)
}

type SheinAPIClientFactory interface {
	NewSheinAPIClient(storeID int64, storeInfo *SheinStoreInfo) *SheinRuntimeAPIClient
}

func NewSheinRuntimeAPIClientWithStoreConfig(storeID int64, storeInfo *SheinRuntimeStoreConfig, cookieProvider SheinRuntimeCookieProvider) *SheinRuntimeAPIClient {
	return sheinclient.NewAPIClientWithStoreConfig(storeID, storeInfo, cookieProvider)
}

func NewSheinRuntimeBaseAPIClient(apiClient *SheinRuntimeAPIClient, storeID int64) *sheinclient.BaseAPIClient {
	if apiClient == nil {
		return nil
	}
	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	return baseAPI
}
