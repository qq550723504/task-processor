package listingkit

import (
	"task-processor/internal/infra/clients/management"
	sheinclient "task-processor/internal/shein/client"
)

type sheinManagedAPIFactory struct {
	client *management.ClientManager
}

func newSheinManagedAPIFactory(client *management.ClientManager) *sheinManagedAPIFactory {
	return &sheinManagedAPIFactory{client: client}
}

func (f *sheinManagedAPIFactory) BuildBaseClient(storeID int64) (*sheinclient.BaseAPIClient, string) {
	if storeID <= 0 {
		return nil, "未提供 shein_store_id，SHEIN 在线解析未启用"
	}
	if f == nil || f.client == nil {
		return nil, "management client 不可用，SHEIN 在线解析未启用"
	}

	apiClient := sheinclient.NewAPIClient(storeID, f.client)
	if !apiClient.HasCookies() {
		return nil, "SHEIN 店铺 cookie 不可用，已降级为离线解析"
	}

	return sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	), ""
}
