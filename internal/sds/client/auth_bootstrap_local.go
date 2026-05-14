package client

import (
	"context"
	"sync"
)

type LocalLoginRequest struct {
	TenantID     string
	Identifier   string
	MerchantName string
	Username     string
	Password     string
	Headless     bool
	ForceLogin   bool
}

type LocalAuthPayload struct {
	AccessToken string
	OutToken    string
	MerchantID  int64
	UserID      int64
	Username    string
	Cookies     []*PersistedCookie
	Source      string
}

type LocalLoginProvider interface {
	TriggerLogin(ctx context.Context, req LocalLoginRequest) error
	LoadAuthState(ctx context.Context, tenantID, identifier string) (*LocalAuthPayload, error)
}

var (
	localLoginProviderMu sync.RWMutex
	localLoginProvider   LocalLoginProvider
)

func ConfigureLocalLoginProvider(provider LocalLoginProvider) {
	localLoginProviderMu.Lock()
	defer localLoginProviderMu.Unlock()
	localLoginProvider = provider
}

func loadLocalLoginProvider() LocalLoginProvider {
	localLoginProviderMu.RLock()
	defer localLoginProviderMu.RUnlock()
	return localLoginProvider
}
