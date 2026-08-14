package httpapi

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

type storeCatalogRepositoryStub struct {
	listingadmin.StoreRepository
	query listingadmin.StoreQuery
	page  *listingadmin.StorePage
	pages []*listingadmin.StorePage
	ctx   context.Context
}

func (s *storeCatalogRepositoryStub) ListStores(ctx context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	s.ctx = ctx
	s.query = query
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
	return s.page, nil
}

func TestSheinStoreCatalogListsOnlyActiveStores(t *testing.T) {
	stub := &storeCatalogRepositoryStub{
		page: &listingadmin.StorePage{
			Items: []listingadmin.Store{
				{ID: 870, TenantID: 505, Platform: "SHEIN", Status: 0},
			},
		},
	}

	options, err := (sheinListingStoreCatalog{repo: stub}).ListStoreOptions(context.Background(), 505)
	if err != nil {
		t.Fatalf("ListStoreOptions error = %v", err)
	}
	if stub.query.Status == nil || *stub.query.Status != 0 {
		t.Fatalf("status query = %v, want active status 0", stub.query.Status)
	}
	if stub.query.Platform != "SHEIN" {
		t.Fatalf("platform query = %q, want SHEIN", stub.query.Platform)
	}
	if !stub.query.ReadAccess {
		t.Fatal("read access query = false, want shared-store read access")
	}
	if len(options) != 1 || options[0].ID != 870 || options[0].Status != 0 {
		t.Fatalf("options = %+v, want active store 870", options)
	}
}

func TestSheinStoreCatalogPropagatesListingKitRoles(t *testing.T) {
	stub := &storeCatalogRepositoryStub{
		page: &listingadmin.StorePage{
			Items: []listingadmin.Store{{ID: 870, TenantID: 505, Platform: "SHEIN", Status: 0}},
		},
	}
	ctx := listingkit.WithRequestRoles(context.Background(), []string{"listingkit_admin"})

	if _, err := (sheinListingStoreCatalog{repo: stub}).ListStoreOptions(ctx, 505); err != nil {
		t.Fatalf("ListStoreOptions error = %v", err)
	}
	roles := listingadmin.RequestRolesFromContext(stub.ctx)
	if len(roles) != 1 || roles[0] != "listingkit_admin" {
		t.Fatalf("listingadmin roles = %v, want listingkit_admin", roles)
	}
}

func TestSheinStoreCatalogListsAllPages(t *testing.T) {
	stub := &storeCatalogRepositoryStub{
		pages: []*listingadmin.StorePage{
			{Items: []listingadmin.Store{{ID: 870, TenantID: 505, Platform: "SHEIN", Status: 0}}, Total: 2, Page: 1, PageSize: 1},
			{Items: []listingadmin.Store{{ID: 871, TenantID: 505, Platform: "SHEIN", Status: 0}}, Total: 2, Page: 2, PageSize: 1},
		},
	}

	options, err := (sheinListingStoreCatalog{repo: stub}).ListStoreOptions(context.Background(), 505)
	if err != nil {
		t.Fatalf("ListStoreOptions error = %v", err)
	}
	if len(options) != 2 || options[0].ID != 870 || options[1].ID != 871 {
		t.Fatalf("options = %+v, want stores from both pages", options)
	}
}
