package listingkit

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinclient "task-processor/internal/shein/client"
)

type stubSheinStoreCatalog struct {
	storeInfo    *SheinStoreInfo
	options      []SheinStoreOption
	err          error
	seenTenantID int64
	seenStoreID  int64
}

func (s *stubSheinStoreCatalog) GetStoreInfo(_ context.Context, tenantID, storeID int64) (*SheinStoreInfo, error) {
	s.seenTenantID = tenantID
	s.seenStoreID = storeID
	if s.err != nil {
		return nil, s.err
	}
	return s.storeInfo, nil
}

func (s *stubSheinStoreCatalog) ListStoreOptions(_ context.Context, _ int64) ([]SheinStoreOption, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]SheinStoreOption(nil), s.options...), nil
}

type stubSheinAPIClientFactory struct {
	client *sheinclient.APIClient
}

func (f stubSheinAPIClientFactory) NewSheinAPIClient(_ int64, _ *SheinStoreInfo) *sheinclient.APIClient {
	return f.client
}

func TestResolveSheinStoreInfoUsesTenantScopedStoreClient(t *testing.T) {
	t.Parallel()

	catalog := &stubSheinStoreCatalog{
		storeInfo: &SheinStoreInfo{
			ID:       869,
			TenantID: 227,
			StoreID:  "869",
			Platform: "shein",
			LoginURL: "sso.geiwohuo.com",
		},
	}

	svc := &service{sheinRuntimeDeps: sheinRuntimeDependencies{storeCatalog: catalog}}
	task := &Task{
		ID:       "task-store-tenant",
		TenantID: "227",
		Request: &GenerateRequest{
			SheinStoreID: 869,
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{
		TenantID: "227",
		UserID:   "user-store-tenant",
	})

	storeInfo, err := svc.resolveSheinStoreInfo(ctx, task)
	if err != nil {
		t.Fatalf("resolveSheinStoreInfo error = %v", err)
	}
	if storeInfo == nil || storeInfo.TenantID != 227 {
		t.Fatalf("store info = %+v, want tenant 227", storeInfo)
	}
	if catalog.seenTenantID != 227 {
		t.Fatalf("tenant id = %d, want 227", catalog.seenTenantID)
	}
	if catalog.seenStoreID != 869 {
		t.Fatalf("store id = %d, want 869", catalog.seenStoreID)
	}
}
