package shein

import (
	"context"

	"github.com/imroc/req/v3"
)

type RuntimeAPIClientFactory interface {
	NewAPIClient(ctx context.Context, storeID int64) RuntimeAPIClient
}

type RuntimeAPIClient interface {
	HasCookies() bool
	ForceRefreshCookies() error
	GetBaseURL() string
	GetTenantID() int64
	GetHTTPClient() *req.Client
}

type runtimeAPIFactory struct {
	clientFactory RuntimeAPIClientFactory
}

func newRuntimeAPIFactory(clientFactory RuntimeAPIClientFactory) *runtimeAPIFactory {
	return &runtimeAPIFactory{clientFactory: clientFactory}
}

func (f *runtimeAPIFactory) BuildBaseClient(ctx context.Context, storeID int64) (*RuntimeBaseAPIClient, string) {
	if storeID <= 0 {
		return nil, "未提供 shein_store_id，SHEIN 在线解析未启用"
	}
	if f == nil || f.clientFactory == nil {
		return nil, "shein runtime client factory 不可用，SHEIN 在线解析未启用"
	}

	apiClient := f.clientFactory.NewAPIClient(ctx, storeID)
	if apiClient == nil {
		return nil, "shein runtime client 不可用，SHEIN 在线解析未启用"
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, "SHEIN 店铺 cookie 不可用，已降级为离线解析"
		}
	}
	if !apiClient.HasCookies() {
		return nil, "SHEIN 店铺 cookie 不可用，已降级为离线解析"
	}

	return newRuntimeBaseAPIClient(apiClient, storeID), ""
}
