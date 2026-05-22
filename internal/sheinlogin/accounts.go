package sheinlogin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
)

type AccountProvider interface {
	ListAccounts(ctx context.Context, tenantID int64) ([]Account, error)
	GetAccount(ctx context.Context, tenantID int64, storeID int64) (*Account, error)
}

type listingAdminAccountStore interface {
	ListStores(ctx context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error)
	GetStore(ctx context.Context, tenantID, id int64) (*listingadmin.Store, error)
}

type ManagementAccountProvider struct {
	client *management.ClientManager
	mu     sync.RWMutex
	cache  map[int64]tenantAccountCache
}

type ListingAdminAccountProvider struct {
	repo  listingAdminAccountStore
	mu    sync.RWMutex
	cache map[int64]tenantAccountCache
}

type tenantAccountCache struct {
	items []Account
	until time.Time
}

func NewManagementAccountProvider(client *management.ClientManager) *ManagementAccountProvider {
	return &ManagementAccountProvider{
		client: client,
		cache:  make(map[int64]tenantAccountCache),
	}
}

func NewListingAdminAccountProvider(repo listingAdminAccountStore) *ListingAdminAccountProvider {
	return &ListingAdminAccountProvider{
		repo:  repo,
		cache: make(map[int64]tenantAccountCache),
	}
}

func (p *ManagementAccountProvider) GetAccount(ctx context.Context, tenantID int64, storeID int64) (*Account, error) {
	accounts, err := p.ListAccounts(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.StoreID == storeID {
			acc := account
			return &acc, nil
		}
	}
	return nil, fmt.Errorf("shein login account not found for tenant %d store %d", tenantID, storeID)
}

func (p *ManagementAccountProvider) ListAccounts(ctx context.Context, tenantID int64) ([]Account, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is required")
	}

	p.mu.RLock()
	entry, exists := p.cache[tenantID]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		cached := append([]Account(nil), entry.items...)
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	entry, exists = p.cache[tenantID]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		return append([]Account(nil), entry.items...), nil
	}
	if p.client == nil {
		return nil, fmt.Errorf("management client is nil")
	}

	storeClient := p.client.GetStoreClientWithTenant(tenantID)
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
			TenantID: tenantID,
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

	p.cache[tenantID] = tenantAccountCache{
		items: append([]Account(nil), items...),
		until: time.Now().Add(5 * time.Second),
	}
	return append([]Account(nil), items...), nil
}

func (p *ListingAdminAccountProvider) GetAccount(ctx context.Context, tenantID int64, storeID int64) (*Account, error) {
	accounts, err := p.ListAccounts(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.StoreID == storeID {
			acc := account
			return &acc, nil
		}
	}
	return nil, fmt.Errorf("shein login account not found for tenant %d store %d", tenantID, storeID)
}

func (p *ListingAdminAccountProvider) ListAccounts(ctx context.Context, tenantID int64) ([]Account, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is required")
	}

	p.mu.RLock()
	entry, exists := p.cache[tenantID]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		cached := append([]Account(nil), entry.items...)
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	entry, exists = p.cache[tenantID]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		return append([]Account(nil), entry.items...), nil
	}
	if p.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is nil")
	}

	page, err := p.repo.ListStores(ctx, listingadmin.StoreQuery{
		TenantID: tenantID,
		Platform: "shein",
		Page:     1,
		PageSize: 200,
	})
	if err != nil {
		return nil, err
	}
	items := make([]Account, 0, len(page.Items))
	for _, store := range page.Items {
		account, ok := mapListingAdminStoreToAccount(&store)
		if ok {
			items = append(items, account)
		}
	}

	p.cache[tenantID] = tenantAccountCache{
		items: append([]Account(nil), items...),
		until: time.Now().Add(5 * time.Second),
	}
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
		LoginURL:  normalizeLoginURL(strings.TrimSpace(store.LoginUrl)),
		Proxy:     strings.TrimSpace(store.Proxy),
		ShopName:  strings.TrimSpace(store.Name),
		Platform:  strings.TrimSpace(store.Platform),
		StoreName: strings.TrimSpace(store.StoreID),
	}, true
}

func mapListingAdminStoreToAccount(store *listingadmin.Store) (Account, bool) {
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
		LoginURL:  normalizeLoginURL(strings.TrimSpace(store.LoginURL)),
		Proxy:     strings.TrimSpace(store.Proxy),
		ShopName:  strings.TrimSpace(store.Name),
		Platform:  strings.TrimSpace(store.Platform),
		StoreName: strings.TrimSpace(store.StoreID),
	}, true
}

func normalizeLoginURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "https://" + value
}
