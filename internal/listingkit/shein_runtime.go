package listingkit

import (
	"context"

	sheinclient "task-processor/internal/shein/client"
)

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
	NewSheinAPIClient(storeID int64, storeInfo *SheinStoreInfo) *sheinclient.APIClient
}
