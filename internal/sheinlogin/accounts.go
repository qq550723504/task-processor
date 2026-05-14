package sheinlogin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
)

type AccountProvider interface {
	ListAccounts(ctx context.Context) ([]Account, error)
	GetAccount(ctx context.Context, storeID int64) (*Account, error)
}

type ManagementAccountProvider struct {
	client *management.ClientManager
	mu     sync.RWMutex
	cache  []Account
	until  time.Time
}

func NewManagementAccountProvider(client *management.ClientManager) *ManagementAccountProvider {
	return &ManagementAccountProvider{client: client}
}

func (p *ManagementAccountProvider) GetAccount(ctx context.Context, storeID int64) (*Account, error) {
	accounts, err := p.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.StoreID == storeID {
			acc := account
			return &acc, nil
		}
	}
	return nil, fmt.Errorf("shein login account not found for store %d", storeID)
}

func (p *ManagementAccountProvider) ListAccounts(ctx context.Context) ([]Account, error) {
	p.mu.RLock()
	if time.Now().Before(p.until) && p.cache != nil {
		cached := append([]Account(nil), p.cache...)
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Now().Before(p.until) && p.cache != nil {
		return append([]Account(nil), p.cache...), nil
	}
	if p.client == nil {
		return nil, fmt.Errorf("management client is nil")
	}

	storeClient := p.client.GetStoreClient()
	if storeClient == nil {
		return nil, fmt.Errorf("management store client is nil")
	}

	var (
		pageNo   = 1
		pageSize = 100
		items    []Account
	)
	for {
		page, err := storeClient.PageStores(&managementapi.StorePageReqDTO{
			Platform: "SHEIN",
			PageNo:   pageNo,
			PageSize: pageSize,
		})
		if err != nil {
			return nil, err
		}
		for _, store := range page.List {
			if store == nil {
				continue
			}
			account, ok := mapStoreToAccount(store)
			if ok {
				items = append(items, account)
			}
		}
		if int64(pageNo*pageSize) >= page.Total || len(page.List) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		pageNo++
	}

	p.cache = append([]Account(nil), items...)
	p.until = time.Now().Add(5 * time.Second)
	return append([]Account(nil), items...), nil
}

func mapStoreToAccount(store *managementapi.StoreRespDTO) (Account, bool) {
	if store == nil {
		return Account{}, false
	}
	if !strings.EqualFold(strings.TrimSpace(store.Platform), "shein") {
		return Account{}, false
	}
	if strings.TrimSpace(store.Username) == "" || strings.TrimSpace(store.Password) == "" {
		return Account{}, false
	}
	return Account{
		StoreID:   store.ID,
		TenantID:  store.TenantID,
		Username:  strings.TrimSpace(store.Username),
		Password:  store.Password,
		LoginURL:  strings.TrimSpace(store.LoginUrl),
		Proxy:     strings.TrimSpace(store.Proxy),
		ShopName:  strings.TrimSpace(store.Name),
		Platform:  strings.TrimSpace(store.Platform),
		StoreName: strings.TrimSpace(store.StoreID),
	}, true
}
