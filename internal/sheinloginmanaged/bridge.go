package sheinloginmanaged

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"task-processor/internal/infra/clients/management"
	"task-processor/internal/ports/managementapi"
	"task-processor/internal/sheinlogin"
)

type storeClientFactory func(tenantID int64) managementapi.StoreAPI

type AccountProvider struct {
	storeClientForTenant storeClientFactory
	mu                   sync.RWMutex
	cache                map[int64]tenantAccountCache
}

type tenantAccountCache struct {
	items []sheinlogin.Account
	until time.Time
}

func NewAccountProvider(client *management.ClientManager) *AccountProvider {
	return &AccountProvider{
		storeClientForTenant: func(tenantID int64) managementapi.StoreAPI {
			if client == nil {
				return nil
			}
			return client.GetStoreClientWithTenant(tenantID)
		},
		cache: make(map[int64]tenantAccountCache),
	}
}

func NewAccountProviderWithStoreClientFactory(factory storeClientFactory) *AccountProvider {
	return &AccountProvider{
		storeClientForTenant: factory,
		cache:                make(map[int64]tenantAccountCache),
	}
}

func (p *AccountProvider) GetAccount(ctx context.Context, tenantID int64, storeID int64) (*sheinlogin.Account, error) {
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

func (p *AccountProvider) ListAccounts(ctx context.Context, tenantID int64) ([]sheinlogin.Account, error) {
	if tenantID <= 0 {
		return nil, fmt.Errorf("tenant id is required")
	}

	p.mu.RLock()
	entry, exists := p.cache[tenantID]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		cached := append([]sheinlogin.Account(nil), entry.items...)
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	entry, exists = p.cache[tenantID]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		return append([]sheinlogin.Account(nil), entry.items...), nil
	}
	if p.storeClientForTenant == nil {
		return nil, fmt.Errorf("management client is nil")
	}

	storeClient := p.storeClientForTenant(tenantID)
	if storeClient == nil {
		return nil, fmt.Errorf("management store client is nil")
	}

	var (
		pageNo   = 1
		pageSize = 100
		items    []sheinlogin.Account
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
		items: append([]sheinlogin.Account(nil), items...),
		until: time.Now().Add(5 * time.Second),
	}
	return append([]sheinlogin.Account(nil), items...), nil
}

func NewStoreSyncClientFactory(client *management.ClientManager) func(tenantID int64) sheinlogin.StoreSyncClient {
	return func(tenantID int64) sheinlogin.StoreSyncClient {
		if client == nil {
			return nil
		}
		return client.GetStoreClientWithTenant(tenantID)
	}
}

func NewDuplicateStoreLookup(client *management.ClientManager) func(ctx context.Context, account sheinlogin.Account, actualStoreID string) (*managementapi.StoreRespDTO, error) {
	return func(ctx context.Context, account sheinlogin.Account, actualStoreID string) (*managementapi.StoreRespDTO, error) {
		if client == nil || strings.TrimSpace(actualStoreID) == "" {
			return nil, nil
		}

		storeClient := client.GetStoreClient()
		if storeClient == nil {
			return nil, nil
		}

		pageNo := 1
		pageSize := 200
		for {
			page, err := storeClient.PageStores(&managementapi.StorePageReqDTO{
				Platform: "SHEIN",
				PageNo:   pageNo,
				PageSize: pageSize,
			})
			if err != nil {
				return nil, err
			}
			for _, item := range page.List {
				if item == nil || item.ID == account.StoreID {
					continue
				}
				if strings.EqualFold(strings.TrimSpace(item.Platform), "shein") && strings.TrimSpace(item.StoreID) == actualStoreID {
					return item, nil
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
		return nil, nil
	}
}

func mapStoreToAccount(store *managementapi.StoreRespDTO) (sheinlogin.Account, bool) {
	if store == nil {
		return sheinlogin.Account{}, false
	}
	if !strings.EqualFold(strings.TrimSpace(store.Platform), "shein") {
		return sheinlogin.Account{}, false
	}
	if strings.TrimSpace(store.Username) == "" || strings.TrimSpace(store.Password) == "" {
		return sheinlogin.Account{}, false
	}
	return sheinlogin.Account{
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

func MapStoreToAccountForTest(store *managementapi.StoreRespDTO) (sheinlogin.Account, bool) {
	return mapStoreToAccount(store)
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
