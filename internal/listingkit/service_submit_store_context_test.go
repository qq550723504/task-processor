package listingkit

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinwarehouse "task-processor/internal/shein/api/warehouse"
)

func TestPickSheinWarehouseCodePrefersMatchingSaleCountry(t *testing.T) {
	t.Parallel()

	warehouses := &sheinwarehouse.WarehouseResponse{
		Data: []sheinwarehouse.Warehouse{
			{WarehouseCode: "WH-EU", SaleCountryList: []string{"DE", "FR"}},
			{WarehouseCode: "WH-US", SaleCountryList: []string{"US", "CA"}},
		},
	}

	if got := pickSheinWarehouseCode(warehouses, "US"); got != "WH-US" {
		t.Fatalf("pick warehouse = %q, want WH-US", got)
	}
}

func TestPickSheinWarehouseCodeFallsBackToFirstWarehouse(t *testing.T) {
	t.Parallel()

	warehouses := &sheinwarehouse.WarehouseResponse{
		Data: []sheinwarehouse.Warehouse{
			{WarehouseCode: "WH-FIRST", SaleCountryList: []string{"DE"}},
			{WarehouseCode: "WH-SECOND", SaleCountryList: []string{"US"}},
		},
	}

	if got := pickSheinWarehouseCode(warehouses, "JP"); got != "WH-FIRST" {
		t.Fatalf("pick warehouse = %q, want WH-FIRST", got)
	}
}

func TestResolveSheinSubmitSettingsUsesStoreProfileFields(t *testing.T) {
	t.Parallel()

	svc := &service{
		storeProfileRepo:    newInMemoryStoreProfileRepository(),
		routingSettingsRepo: newInMemoryStoreRoutingSettingsRepository(),
		sheinSettings: SheinSettings{
			DefaultStoreID:    700,
			Site:              "US",
			WarehouseCode:     "DEFAULT",
			DefaultStock:      100,
			DefaultSubmitMode: "publish",
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "404", UserID: "user-d"})
	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:           902,
		Enabled:           true,
		Priority:          1,
		Site:              "GB",
		WarehouseCode:     "WH-GB-1",
		DefaultStock:      66,
		DefaultSubmitMode: "save_draft",
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}

	settings := svc.resolveSheinSubmitSettings(ctx, &Task{
		TenantID: "404",
		Request: &GenerateRequest{
			SheinStoreID: 902,
		},
	})
	if settings.DefaultStoreID != 902 {
		t.Fatalf("default store id = %d, want 902", settings.DefaultStoreID)
	}
	if settings.Site != "GB" || settings.WarehouseCode != "WH-GB-1" || settings.DefaultStock != 66 || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("settings = %+v, want profile-backed settings", settings)
	}
}

func TestSheinSubmitPreferredWarehouseCodeUsesFirstCSVItem(t *testing.T) {
	t.Parallel()

	if got := sheinSubmitPreferredWarehouseCode(SheinSettings{WarehouseCode: "WH-CA-1,WH-US-1"}); got != "WH-CA-1" {
		t.Fatalf("preferred warehouse = %q, want WH-CA-1", got)
	}
}

func TestResolveSheinSubmitSettingsPrefersTaskSnapshotOverCurrentProfiles(t *testing.T) {
	t.Parallel()

	svc := &service{
		storeProfileRepo:    newInMemoryStoreProfileRepository(),
		routingSettingsRepo: newInMemoryStoreRoutingSettingsRepository(),
		sheinSettings: SheinSettings{
			DefaultStoreID:    700,
			Site:              "US",
			WarehouseCode:     "DEFAULT",
			DefaultStock:      100,
			DefaultSubmitMode: "publish",
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "405", UserID: "user-e"})
	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:           903,
		Enabled:           true,
		Priority:          1,
		Site:              "US",
		WarehouseCode:     "WH-US-9",
		DefaultStock:      11,
		DefaultSubmitMode: "publish",
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}

	settings := svc.resolveSheinSubmitSettings(ctx, &Task{
		TenantID: "405",
		Request:  &GenerateRequest{},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:           902,
			Site:              "GB",
			WarehouseCode:     "WH-GB-7",
			DefaultStock:      66,
			DefaultSubmitMode: "save_draft",
			Strategy:          "country",
			Reason:            "snapshot persisted at task creation",
			MatchedProfileID:  12,
		},
	})
	if settings.DefaultStoreID != 902 {
		t.Fatalf("default store id = %d, want snapshot store 902", settings.DefaultStoreID)
	}
	if settings.Site != "GB" || settings.WarehouseCode != "WH-GB-7" || settings.DefaultStock != 66 || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("settings = %+v, want snapshot-backed settings", settings)
	}
}

func TestApplySubmitSettingsProfileOverlaysProfileFields(t *testing.T) {
	t.Parallel()

	settings := applySubmitSettingsProfile(SheinSettings{
		DefaultStoreID:    700,
		Site:              "US",
		WarehouseCode:     "WH-US-1",
		DefaultStock:      100,
		DefaultSubmitMode: "publish",
	}, &ListingKitStoreProfile{
		StoreID:           902,
		Site:              "GB",
		WarehouseCode:     "WH-GB-1",
		DefaultStock:      66,
		DefaultSubmitMode: "save_draft",
	})

	if settings.DefaultStoreID != 902 {
		t.Fatalf("default store id = %d, want 902", settings.DefaultStoreID)
	}
	if settings.Site != "GB" || settings.WarehouseCode != "WH-GB-1" || settings.DefaultStock != 66 || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("settings = %+v, want profile-backed settings", settings)
	}
}

func TestApplySubmitSettingsTaskRequestPrefersCountry(t *testing.T) {
	t.Parallel()

	settings := applySubmitSettingsTaskRequest(SheinSettings{
		Site:          "GB",
		WarehouseCode: "WH-GB-1",
	}, &Task{
		Request: &GenerateRequest{
			Country: "us",
		},
	})

	if settings.Site != "US" {
		t.Fatalf("site = %q, want US", settings.Site)
	}
	if settings.WarehouseCode != "WH-GB-1" {
		t.Fatalf("warehouse code = %q, want original warehouse", settings.WarehouseCode)
	}
}

func TestApplySubmitWarehouseOverrideUsesNonEmptyWarehouseCode(t *testing.T) {
	t.Parallel()

	settings := applySubmitWarehouseOverride(SheinSettings{
		Site:          "US",
		WarehouseCode: "WH-US-1",
	}, "WH-US-9")
	if settings.WarehouseCode != "WH-US-9" {
		t.Fatalf("warehouse code = %q, want WH-US-9", settings.WarehouseCode)
	}

	settings = applySubmitWarehouseOverride(settings, "")
	if settings.WarehouseCode != "WH-US-9" {
		t.Fatalf("warehouse code = %q, want preserved WH-US-9", settings.WarehouseCode)
	}
}
