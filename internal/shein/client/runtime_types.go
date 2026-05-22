package client

import "context"

type StoreConfig struct {
	ID       int64
	TenantID int64
	StoreID  string
	Name     string
	Platform string
	Region   string
	LoginURL string
	Proxy    string
}

type CookieLookupResult struct {
	TenantID   int64
	CookieJSON string
}

type CookieProvider interface {
	GetCookie(ctx context.Context, storeID int64) (*CookieLookupResult, error)
}

type StoreConfigProvider interface {
	GetStoreConfig(ctx context.Context, storeID int64) (*StoreConfig, error)
}
