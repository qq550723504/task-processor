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
		storeProfileRepo:      newInMemoryStoreProfileRepository(),
		routingSettingsRepo:   newInMemoryStoreRoutingSettingsRepository(),
		sheinManagementClient: nil,
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

func TestStoreProfileServiceUpdatesRoutingSettings(t *testing.T) {
	t.Parallel()

	svc := &service{
		storeProfileRepo:    newInMemoryStoreProfileRepository(),
		routingSettingsRepo: newInMemoryStoreRoutingSettingsRepository(),
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "202", UserID: "user-b"})

	saved, err := svc.UpdateSheinStoreRoutingSettings(ctx, &ListingKitStoreRoutingSettings{
		SelectionStrategy:   "priority",
		FallbackStoreID:     870,
		AllowManualOverride: true,
		AllowFallback:       true,
	})
	if err != nil {
		t.Fatalf("UpdateSheinStoreRoutingSettings error = %v", err)
	}
	if saved.TenantID != 202 {
		t.Fatalf("tenant id = %d, want 202", saved.TenantID)
	}
	if saved.SelectionStrategy != "priority" {
		t.Fatalf("selection strategy = %q, want priority", saved.SelectionStrategy)
	}

	current, err := svc.GetSheinStoreRoutingSettings(ctx)
	if err != nil {
		t.Fatalf("GetSheinStoreRoutingSettings error = %v", err)
	}
	if current.FallbackStoreID != 870 || !current.AllowFallback || !current.AllowManualOverride {
		t.Fatalf("routing settings = %+v, want persisted routing settings", current)
	}
}

func TestResolveSheinStoreIDUsesProfilePriorityAndFallback(t *testing.T) {
	t.Parallel()

	svc := &service{
		storeProfileRepo:    newInMemoryStoreProfileRepository(),
		routingSettingsRepo: newInMemoryStoreRoutingSettingsRepository(),
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "303", UserID: "user-c"})

	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:    900,
		Enabled:    true,
		Priority:   20,
		IsFallback: true,
	})
	if err != nil {
		t.Fatalf("fallback profile upsert error = %v", err)
	}
	_, err = svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:  901,
		Enabled:  true,
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("priority profile upsert error = %v", err)
	}

	storeID, err := svc.resolveSheinStoreID(ctx, &Task{
		TenantID: "303",
		Request:  &GenerateRequest{},
	})
	if err != nil {
		t.Fatalf("resolveSheinStoreID error = %v", err)
	}
	if storeID != 901 {
		t.Fatalf("resolved store id = %d, want 901", storeID)
	}

	settings, err := svc.UpdateSheinStoreRoutingSettings(ctx, &ListingKitStoreRoutingSettings{
		SelectionStrategy: "priority",
		FallbackStoreID:   900,
		AllowFallback:     true,
	})
	if err != nil {
		t.Fatalf("UpdateSheinStoreRoutingSettings error = %v", err)
	}
	if settings.FallbackStoreID != 900 {
		t.Fatalf("fallback store id = %d, want 900", settings.FallbackStoreID)
	}
}

func TestResolveSheinStoreIDUsesMatchRulesBeforeFallback(t *testing.T) {
	t.Parallel()

	svc := &service{
		storeProfileRepo:    newInMemoryStoreProfileRepository(),
		routingSettingsRepo: newInMemoryStoreRoutingSettingsRepository(),
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "404", UserID: "user-d"})

	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:    910,
		Enabled:    true,
		Priority:   50,
		IsFallback: true,
	})
	if err != nil {
		t.Fatalf("fallback profile upsert error = %v", err)
	}
	_, err = svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:  911,
		Enabled:  true,
		Priority: 10,
		MatchRules: []ListingKitStoreMatchRule{
			{Kind: "country", Values: []string{"US"}},
		},
	})
	if err != nil {
		t.Fatalf("country profile upsert error = %v", err)
	}
	_, err = svc.UpdateSheinStoreRoutingSettings(ctx, &ListingKitStoreRoutingSettings{
		SelectionStrategy: "country",
		FallbackStoreID:   910,
		AllowFallback:     true,
	})
	if err != nil {
		t.Fatalf("UpdateSheinStoreRoutingSettings error = %v", err)
	}

	storeID, err := svc.resolveSheinStoreID(ctx, &Task{
		TenantID: "404",
		Request: &GenerateRequest{
			Country: "US",
		},
	})
	if err != nil {
		t.Fatalf("resolveSheinStoreID error = %v", err)
	}
	if storeID != 911 {
		t.Fatalf("resolved store id = %d, want 911", storeID)
	}

	fallbackStoreID, err := svc.resolveSheinStoreID(ctx, &Task{
		TenantID: "404",
		Request: &GenerateRequest{
			Country: "CA",
		},
	})
	if err != nil {
		t.Fatalf("resolveSheinStoreID fallback error = %v", err)
	}
	if fallbackStoreID != 910 {
		t.Fatalf("fallback resolved store id = %d, want 910", fallbackStoreID)
	}
}

func TestStoreProfileServiceResolvesLegacyTenantIDFromMappedZitadelTenant(t *testing.T) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(storeProfileLegacyTenantResolver{
		mapping: map[string]int64{"373211199677923496": 227},
	})
	t.Cleanup(restore)

	svc := &service{
		storeProfileRepo:    newInMemoryStoreProfileRepository(),
		routingSettingsRepo: newInMemoryStoreRoutingSettingsRepository(),
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
