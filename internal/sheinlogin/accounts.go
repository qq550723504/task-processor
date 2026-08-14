package sheinlogin

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

type AccountProvider interface {
	ListAccounts(ctx context.Context, tenantID int64) ([]Account, error)
	GetAccount(ctx context.Context, tenantID int64, storeID int64) (*Account, error)
}

type listingAdminAccountStore interface {
	ListStores(ctx context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error)
	GetStore(ctx context.Context, tenantID, id int64) (*listingadmin.Store, error)
}

type ListingAdminAccountProvider struct {
	repo  listingAdminAccountStore
	mu    sync.RWMutex
	cache map[accountCacheKey]tenantAccountCache
}

type accountCacheKey struct {
	tenantID int64
	userID   string
	roles    string
}

type tenantAccountCache struct {
	items []Account
	until time.Time
}

func NewListingAdminAccountProvider(repo listingAdminAccountStore) *ListingAdminAccountProvider {
	return &ListingAdminAccountProvider{
		repo:  repo,
		cache: make(map[accountCacheKey]tenantAccountCache),
	}
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
	cacheKey := accountCacheKeyForContext(ctx, tenantID)

	p.mu.RLock()
	entry, exists := p.cache[cacheKey]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		cached := append([]Account(nil), entry.items...)
		p.mu.RUnlock()
		return cached, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()
	entry, exists = p.cache[cacheKey]
	if exists && time.Now().Before(entry.until) && entry.items != nil {
		return append([]Account(nil), entry.items...), nil
	}
	if p.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is nil")
	}

	queryCtx := listingadmin.WithRequestRoles(ctx, listingkit.RequestRolesFromContext(ctx))
	const pageSize = 200
	items := make([]Account, 0, pageSize)
	for pageNumber := 1; ; pageNumber++ {
		page, err := p.repo.ListStores(queryCtx, listingadmin.StoreQuery{
			TenantID:   tenantID,
			Platform:   "SHEIN",
			ReadAccess: true,
			Page:       pageNumber,
			PageSize:   pageSize,
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.Items) == 0 {
			break
		}
		for _, store := range page.Items {
			account, ok := mapListingAdminStoreToAccount(&store)
			if ok {
				items = append(items, account)
			}
		}
		responsePageSize := page.PageSize
		if responsePageSize <= 0 {
			responsePageSize = pageSize
		}
		if int64(pageNumber*responsePageSize) >= page.Total || len(page.Items) < responsePageSize {
			break
		}
	}

	p.cache[cacheKey] = tenantAccountCache{
		items: append([]Account(nil), items...),
		until: time.Now().Add(5 * time.Second),
	}
	return append([]Account(nil), items...), nil
}

func accountCacheKeyForContext(ctx context.Context, tenantID int64) accountCacheKey {
	roles := listingkit.RequestRolesFromContext(ctx)
	sort.Strings(roles)
	return accountCacheKey{
		tenantID: tenantID,
		userID:   listingkit.RequestUserIDFromContext(ctx),
		roles:    strings.Join(roles, "\x00"),
	}
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
