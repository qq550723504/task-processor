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

type recordingSheinAPIClientFactory struct {
	client *sheinclient.APIClient
	calls  int
}

func (f *recordingSheinAPIClientFactory) NewSheinAPIClient(_ int64, _ *SheinStoreInfo) *sheinclient.APIClient {
	f.calls++
	return f.client
}

type rejectingStoreAccessValidator struct {
	err error
}

func (v rejectingStoreAccessValidator) ValidateStoreAccess(context.Context, int64, int64, string) (StoreAccess, error) {
	return StoreAccess{}, v.err
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

	svc := &service{sheinSharedDeps: sheinSharedDependencies{
		storeCatalog:         catalog,
		storeAccessValidator: &storeAccessValidatorStub{},
	}}
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

func TestNewSheinAPIClientRejectsStaleStoreSnapshotBeforeCreatingClient(t *testing.T) {
	t.Parallel()

	factory := &recordingSheinAPIClientFactory{}
	svc := &service{sheinSharedDeps: sheinSharedDependencies{
		storeCatalog: &stubSheinStoreCatalog{storeInfo: &SheinStoreInfo{
			ID:       869,
			TenantID: 227,
			Platform: "shein",
		}},
		storeAccessValidator: rejectingStoreAccessValidator{err: NewStoreAccessError(StoreAccessDisabled, "store is disabled")},
		apiClientFactory:     factory,
	}}
	task := &Task{
		TenantID: "227",
		Request:  &GenerateRequest{},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID: 869,
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "227", UserID: "user-store-tenant"})

	_, _, err := svc.newSheinAPIClient(ctx, task)
	if got := StoreAccessErrorCode(err); got != StoreAccessStale {
		t.Fatalf("store access error = %q, want %q (err=%v)", got, StoreAccessStale, err)
	}
	if factory.calls != 0 {
		t.Fatalf("api client factory calls = %d, want 0", factory.calls)
	}
}

func TestNewSheinAPIClientRejectsSnapshotWhenCatalogNoLongerMatchesStore(t *testing.T) {
	t.Parallel()

	factory := &recordingSheinAPIClientFactory{}
	svc := &service{sheinSharedDeps: sheinSharedDependencies{
		storeCatalog: &stubSheinStoreCatalog{storeInfo: &SheinStoreInfo{
			ID:       869,
			TenantID: 228,
			Platform: "shein",
		}},
		storeAccessValidator: &storeAccessValidatorStub{},
		apiClientFactory:     factory,
	}}
	task := &Task{
		TenantID: "227",
		Request:  &GenerateRequest{},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID: 869,
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "227", UserID: "user-store-tenant"})

	_, _, err := svc.newSheinAPIClient(ctx, task)
	if got := StoreAccessErrorCode(err); got != StoreAccessStale {
		t.Fatalf("store access error = %q, want %q (err=%v)", got, StoreAccessStale, err)
	}
	if factory.calls != 0 {
		t.Fatalf("api client factory calls = %d, want 0", factory.calls)
	}
}

func TestResolveSheinStoreIDPrefersPersistedSnapshotOverMutableRequest(t *testing.T) {
	t.Parallel()

	resolver := buildSubmitRuntimeContextResolver(&service{})
	storeID, err := resolver.resolveStoreID(context.Background(), &Task{
		Request: &GenerateRequest{SheinStoreID: 870},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID: 869,
		},
	})
	if err != nil {
		t.Fatalf("resolve store id: %v", err)
	}
	if storeID != 869 {
		t.Fatalf("store id = %d, want snapshot store 869", storeID)
	}
}
