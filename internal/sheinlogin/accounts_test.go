package sheinlogin

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

type stubListingAdminAccountStore struct {
	items []listingadmin.Store
	pages []*listingadmin.StorePage
	query listingadmin.StoreQuery
	roles []string
	calls int
}

func (s *stubListingAdminAccountStore) ListStores(ctx context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	s.calls++
	s.query = query
	s.roles = listingadmin.RequestRolesFromContext(ctx)
	if len(s.pages) > 0 {
		index := query.Page - 1
		if index < 0 {
			index = 0
		}
		if index >= len(s.pages) {
			return nil, nil
		}
		return s.pages[index], nil
	}
	items := make([]listingadmin.Store, 0, len(s.items))
	for _, item := range s.items {
		if query.TenantID > 0 && item.TenantID != query.TenantID {
			continue
		}
		if query.Platform != "" && item.Platform != query.Platform {
			continue
		}
		items = append(items, item)
	}
	return &listingadmin.StorePage{Items: items, Total: int64(len(items)), Page: 1, PageSize: len(items)}, nil
}

func TestListingAdminAccountProviderLoadsAccountsAcrossAllStorePages(t *testing.T) {
	repo := &stubListingAdminAccountStore{pages: []*listingadmin.StorePage{
		{Items: []listingadmin.Store{{
			ID: 12, TenantID: 7, Platform: "SHEIN", Username: "first", Password: "secret",
		}}, Total: 2, Page: 1, PageSize: 1},
		{Items: []listingadmin.Store{{
			ID: 13, TenantID: 7, Platform: "SHEIN", Username: "second", Password: "secret",
		}}, Total: 2, Page: 2, PageSize: 1},
	}}
	provider := NewListingAdminAccountProvider(repo)

	account, err := provider.GetAccount(context.Background(), 7, 13)
	if err != nil {
		t.Fatalf("GetAccount() error = %v", err)
	}
	if account.StoreID != 13 {
		t.Fatalf("account store id = %d, want store 13", account.StoreID)
	}
	if repo.calls != 2 {
		t.Fatalf("ListStores calls = %d, want both pages", repo.calls)
	}
}

func TestListingAdminAccountProviderDoesNotShareOwnerScopedCacheAcrossUsers(t *testing.T) {
	repo := &stubListingAdminAccountStore{items: []listingadmin.Store{{
		ID: 12, TenantID: 7, Platform: "SHEIN", Username: "demo-user", Password: "secret",
	}}}
	provider := NewListingAdminAccountProvider(repo)
	ctxA := listingkit.WithRequestIdentity(context.Background(), listingkit.RequestIdentity{TenantID: "7", UserID: "user-a"})
	ctxB := listingkit.WithRequestIdentity(context.Background(), listingkit.RequestIdentity{TenantID: "7", UserID: "user-b"})

	if _, err := provider.ListAccounts(ctxA, 7); err != nil {
		t.Fatalf("ListAccounts(user-a): %v", err)
	}
	if _, err := provider.ListAccounts(ctxB, 7); err != nil {
		t.Fatalf("ListAccounts(user-b): %v", err)
	}
	if repo.calls != 2 {
		t.Fatalf("ListStores calls = %d, want one scoped lookup per user", repo.calls)
	}
}

func (s *stubListingAdminAccountStore) GetStore(_ context.Context, tenantID, id int64) (*listingadmin.Store, error) {
	for _, item := range s.items {
		if item.TenantID == tenantID && item.ID == id {
			store := item
			return &store, nil
		}
	}
	return nil, listingadmin.ErrStoreNotFound
}

func (s *stubListingAdminAccountStore) CreateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected CreateStore")
}
func (s *stubListingAdminAccountStore) UpdateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected UpdateStore")
}
func (s *stubListingAdminAccountStore) UpdateStoreStatus(context.Context, int64, int64, int16, string) (*listingadmin.Store, error) {
	panic("unexpected UpdateStoreStatus")
}
func (s *stubListingAdminAccountStore) DeleteStore(context.Context, int64, int64) error {
	panic("unexpected DeleteStore")
}
func (s *stubListingAdminAccountStore) ListDeletedStores(context.Context, int64) ([]listingadmin.Store, error) {
	panic("unexpected ListDeletedStores")
}
func (s *stubListingAdminAccountStore) RestoreStore(context.Context, int64, int64) (*listingadmin.Store, error) {
	panic("unexpected RestoreStore")
}
func (s *stubListingAdminAccountStore) PermanentlyDeleteStore(context.Context, int64, int64) error {
	panic("unexpected PermanentlyDeleteStore")
}
func (s *stubListingAdminAccountStore) ExtendStoreValidity(context.Context, int64, int64, int) (*listingadmin.Store, error) {
	panic("unexpected ExtendStoreValidity")
}

func TestListingAdminAccountProviderLoadsSheinAccountsFromRepository(t *testing.T) {
	provider := NewListingAdminAccountProvider(&stubListingAdminAccountStore{
		items: []listingadmin.Store{
			{
				ID:       12,
				TenantID: 7,
				Platform: "SHEIN",
				Username: "demo-user",
				Password: "secret",
				LoginURL: "sellerhub.shein.com",
				Proxy:    "http://127.0.0.1:8080",
				Name:     "Demo Shop",
				StoreID:  "SHEIN-12",
			},
			{
				ID:       99,
				TenantID: 7,
				Platform: "TEMU",
				Username: "ignored",
				Password: "ignored",
			},
		},
	})

	accounts, err := provider.ListAccounts(context.Background(), 7)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("account count = %d, want 1", len(accounts))
	}
	if accounts[0].StoreID != 12 || accounts[0].TenantID != 7 {
		t.Fatalf("unexpected account identity: %+v", accounts[0])
	}
	if accounts[0].LoginURL != "https://sellerhub.shein.com" {
		t.Fatalf("expected normalized login url, got %q", accounts[0].LoginURL)
	}
	if !provider.repo.(*stubListingAdminAccountStore).query.ReadAccess {
		t.Fatal("ListAccounts query ReadAccess = false, want shared-store read access")
	}
}

func TestListingAdminAccountProviderBridgesAuthenticatedIdentityRoles(t *testing.T) {
	repo := &stubListingAdminAccountStore{items: []listingadmin.Store{{
		ID: 12, TenantID: 7, Platform: "SHEIN", Username: "admin-store", Password: "secret",
	}}}
	provider := NewListingAdminAccountProvider(repo)
	ctx := listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{
		TenantID: "7",
		UserID:   "tenant-admin",
		Roles:    []string{"listingkit_admin"},
	})

	accounts, err := provider.ListAccounts(ctx, 7)
	if err != nil {
		t.Fatalf("ListAccounts: %v", err)
	}
	if len(accounts) != 1 || accounts[0].StoreID != 12 {
		t.Fatalf("accounts = %+v, want tenant-wide SHEIN account", accounts)
	}
	if len(repo.roles) != 1 || repo.roles[0] != "listingkit_admin" {
		t.Fatalf("repository roles = %v, want authenticated listingkit_admin role", repo.roles)
	}
}

func TestListingAdminAccountProviderAuthenticatedIdentityCacheIncludesUserAndRoles(t *testing.T) {
	repo := &stubListingAdminAccountStore{items: []listingadmin.Store{{
		ID: 12, TenantID: 7, Platform: "SHEIN", Username: "demo-user", Password: "secret",
	}}}
	provider := NewListingAdminAccountProvider(repo)
	adminCtx := listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{
		TenantID: "7",
		UserID:   "tenant-admin",
		Roles:    []string{"listingkit_admin"},
	})
	userCtx := listingkit.WithAuthenticatedIdentity(context.Background(), listingkit.AuthenticatedIdentity{
		TenantID: "7",
		UserID:   "regular-user",
		Roles:    []string{"listingkit_viewer"},
	})

	if _, err := provider.ListAccounts(adminCtx, 7); err != nil {
		t.Fatalf("ListAccounts(admin): %v", err)
	}
	if _, err := provider.ListAccounts(userCtx, 7); err != nil {
		t.Fatalf("ListAccounts(user): %v", err)
	}
	if repo.calls != 2 {
		t.Fatalf("ListStores calls = %d, want separate cache entries", repo.calls)
	}
}
