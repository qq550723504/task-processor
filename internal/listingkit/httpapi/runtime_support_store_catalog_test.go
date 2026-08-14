package httpapi

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
)

type storeCatalogRepositoryStub struct {
	listingadmin.StoreRepository
	query listingadmin.StoreQuery
	page  *listingadmin.StorePage
}

func (s *storeCatalogRepositoryStub) ListStores(_ context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	s.query = query
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
