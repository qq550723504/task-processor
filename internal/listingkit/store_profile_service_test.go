package listingkit

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/tenantbridge"
)

func TestStoreProfileServiceUpsertListAndDelete(t *testing.T) {
	t.Parallel()

	svc := &service{
		mirrors: serviceDependencyMirrors{storeProfileRepo: newInMemoryStoreProfileRepository()},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "101", UserID: "user-a"})

	saved, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:           869,
		Enabled:           true,
		Priority:          10,
		Site:              "us",
		WarehouseCode:     "WH-US-1",
		DefaultStock:      88,
		DefaultSubmitMode: "publish",
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}
	if saved.ID <= 0 {
		t.Fatalf("profile id = %d, want > 0", saved.ID)
	}
	if saved.TenantID != 101 {
		t.Fatalf("tenant id = %d, want 101", saved.TenantID)
	}
	if saved.Site != "US" {
		t.Fatalf("site = %q, want US", saved.Site)
	}

	items, err := svc.ListSheinStoreProfiles(ctx)
	if err != nil {
		t.Fatalf("ListSheinStoreProfiles error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("profile count = %d, want 1", len(items))
	}
	if items[0].StoreID != 869 || items[0].WarehouseCode != "WH-US-1" {
		t.Fatalf("profiles = %+v, want persisted store profile", items)
	}

	if err := svc.DeleteSheinStoreProfile(ctx, saved.ID); err != nil {
		t.Fatalf("DeleteSheinStoreProfile error = %v", err)
	}

	items, err = svc.ListSheinStoreProfiles(ctx)
	if err != nil {
		t.Fatalf("ListSheinStoreProfiles after delete error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("profile count after delete = %d, want 0", len(items))
	}
}

func TestResolveSheinStoreIDUsesExplicitRequestStore(t *testing.T) {
	t.Parallel()

	svc := &service{
		mirrors: serviceDependencyMirrors{storeProfileRepo: newInMemoryStoreProfileRepository()},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "303", UserID: "user-c"})

	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:  901,
		Enabled:  true,
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("profile upsert error = %v", err)
	}

	storeID, err := svc.resolveSheinStoreID(ctx, &Task{
		TenantID: "303",
		Request: &GenerateRequest{
			SheinStoreID: 901,
		},
	})
	if err != nil {
		t.Fatalf("resolveSheinStoreID error = %v", err)
	}
	if storeID != 901 {
		t.Fatalf("resolved store id = %d, want 901", storeID)
	}
}

func TestResolveSheinStoreIDUsesSnapshotWhenRequestStoreMissing(t *testing.T) {
	t.Parallel()

	svc := &service{
		mirrors: serviceDependencyMirrors{storeProfileRepo: newInMemoryStoreProfileRepository()},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "404", UserID: "user-d"})

	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		ID:       77,
		StoreID:  911,
		Enabled:  true,
		Priority: 10,
		Site:     "US",
	})
	if err != nil {
		t.Fatalf("profile upsert error = %v", err)
	}

	storeID, err := svc.resolveSheinStoreID(ctx, &Task{
		TenantID: "404",
		Request:  &GenerateRequest{},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:          911,
			Site:             "US",
			Strategy:         "manual",
			Reason:           "任务显式指定了 SHEIN 店铺。",
			MatchedProfileID: 77,
		},
	})
	if err != nil {
		t.Fatalf("resolveSheinStoreID error = %v", err)
	}
	if storeID != 911 {
		t.Fatalf("resolved store id = %d, want 911", storeID)
	}
}

func TestStoreProfileServiceResolvesLegacyTenantIDFromMappedZitadelTenant(t *testing.T) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(storeProfileLegacyTenantResolver{
		mapping: map[string]int64{"373211199677923496": 227},
	})
	t.Cleanup(restore)

	svc := &service{
		mirrors: serviceDependencyMirrors{storeProfileRepo: newInMemoryStoreProfileRepository()},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "373211199677923496", UserID: "user-z"})

	saved, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:  999,
		Enabled:  true,
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}
	if saved.TenantID != 227 {
		t.Fatalf("tenant id = %d, want 227", saved.TenantID)
	}

	items, err := svc.ListSheinStoreProfiles(ctx)
	if err != nil {
		t.Fatalf("ListSheinStoreProfiles error = %v", err)
	}
	if len(items) != 1 || items[0].TenantID != 227 {
		t.Fatalf("items = %+v, want mapped legacy tenant 227 profile", items)
	}
}

type storeProfileLegacyTenantResolver struct {
	mapping map[string]int64
}

func (s storeProfileLegacyTenantResolver) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	value, ok := s.mapping[tenantID]
	return value, ok, nil
}
