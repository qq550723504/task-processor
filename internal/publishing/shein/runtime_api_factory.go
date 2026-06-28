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
