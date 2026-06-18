package sheinmanaged

import (
	"fmt"
	"sync"
	"time"

	"task-processor/internal/infra/clients/management"
	sheinclient "task-processor/internal/shein/client"
	sheinmanagedclient "task-processor/internal/shein/managedclient"
)

type apiFactory struct {
	client *management.ClientManager
}

type baseAPICacheEntry struct {
	baseAPI   *sheinclient.BaseAPIClient
	note      string
	expiresAt time.Time
}

var baseAPICache sync.Map

func newAPIFactory(client *management.ClientManager) *apiFactory {
	return &apiFactory{client: client}
}

func (f *apiFactory) BuildBaseClient(storeID int64) (*sheinclient.BaseAPIClient, string) {
	if storeID <= 0 {
		return nil, "未提供 shein_store_id，SHEIN 在线解析未启用"
	}
	if f == nil || f.client == nil {
		return nil, "management client 不可用，SHEIN 在线解析未启用"
	}

	cacheKey := fmt.Sprintf("%p:%d", f.client, storeID)
	if cached, ok := baseAPICache.Load(cacheKey); ok {
		entry, ok := cached.(baseAPICacheEntry)
		if ok && time.Now().Before(entry.expiresAt) {
			return entry.baseAPI, entry.note
		}
		baseAPICache.Delete(cacheKey)
	}

	apiClient := sheinmanagedclient.NewAPIClient(storeID, f.client)
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err == nil && apiClient.HasCookies() {
			baseAPICache.Delete(cacheKey)
		}
	}
	if !apiClient.HasCookies() {
		note := "SHEIN 店铺 cookie 不可用，已降级为离线解析"
		baseAPICache.Store(cacheKey, baseAPICacheEntry{
			note:      note,
			expiresAt: time.Now().Add(30 * time.Second),
		})
		return nil, note
	}

	baseAPI := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	baseAPI.SetAuthRefreshFunc(apiClient.ForceRefreshCookies)
	baseAPICache.Store(cacheKey, baseAPICacheEntry{
		baseAPI:   baseAPI,
		expiresAt: time.Now().Add(5 * time.Minute),
	})
	return baseAPI, ""
}
