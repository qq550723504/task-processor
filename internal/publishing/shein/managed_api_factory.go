package shein

import (
	"fmt"
	"sync"
	"task-processor/internal/infra/clients/management"
	sheinclient "task-processor/internal/shein/client"
	"time"
)

type managedAPIFactory struct {
	client *management.ClientManager
}

type managedBaseAPICacheEntry struct {
	baseAPI   *sheinclient.BaseAPIClient
	note      string
	expiresAt time.Time
}

var managedBaseAPICache sync.Map

func newManagedAPIFactory(client *management.ClientManager) *managedAPIFactory {
	return &managedAPIFactory{client: client}
}

func (f *managedAPIFactory) BuildBaseClient(storeID int64) (*sheinclient.BaseAPIClient, string) {
	if storeID <= 0 {
		return nil, "未提供 shein_store_id，SHEIN 在线解析未启用"
	}
	if f == nil || f.client == nil {
		return nil, "management client 不可用，SHEIN 在线解析未启用"
	}

	cacheKey := fmt.Sprintf("%p:%d", f.client, storeID)
	if cached, ok := managedBaseAPICache.Load(cacheKey); ok {
		entry, ok := cached.(managedBaseAPICacheEntry)
		if ok && time.Now().Before(entry.expiresAt) {
			return entry.baseAPI, entry.note
		}
		managedBaseAPICache.Delete(cacheKey)
	}

	apiClient := sheinclient.NewAPIClient(storeID, f.client)
	if !apiClient.HasCookies() {
		note := "SHEIN 店铺 cookie 不可用，已降级为离线解析"
		managedBaseAPICache.Store(cacheKey, managedBaseAPICacheEntry{
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
	managedBaseAPICache.Store(cacheKey, managedBaseAPICacheEntry{
		baseAPI:   baseAPI,
		expiresAt: time.Now().Add(5 * time.Minute),
	})
	return baseAPI, ""
}
