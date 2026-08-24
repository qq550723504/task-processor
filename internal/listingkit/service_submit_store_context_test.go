package listingkit

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
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

func TestResolveSheinSubmitSettingsUsesExplicitTaskStoreAndProfileFields(t *testing.T) {
	t.Parallel()

	storeProfileRepo := newInMemoryStoreProfileRepository()
	svc := &service{
		adminDeps:      adminDependencies{storeProfileRepo: storeProfileRepo},
		submissionDeps: submissionDependencies{storeProfileRepo: storeProfileRepo},
		sheinSettings: SheinSettings{
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
		Pricing: sheinpub.PricingRule{
			SourceCurrency:   "CNY",
			TargetCurrency:   "GBP",
			ExchangeRate:     9.1,
			MarkupMultiplier: 1.4,
			MinimumPrice:     10,
			RoundTo:          0.05,
			PriceEnding:      0.99,
		},
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}

	task := &Task{
		TenantID: "404",
		Request: &GenerateRequest{
			SheinStoreID: 902,
		},
	}
	settings := svc.resolveSheinSubmitSettings(ctx, task)
	storeID, err := svc.resolveSheinStoreID(ctx, task)
	if err != nil {
		t.Fatalf("resolveSheinStoreID error = %v", err)
	}
	if storeID != 902 {
		t.Fatalf("resolved store id = %d, want 902", storeID)
	}
	if settings.Site != "GB" || settings.WarehouseCode != "WH-GB-1" || settings.DefaultStock != 66 || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("settings = %+v, want profile-backed settings", settings)
	}
	if settings.Pricing != (sheinpub.PricingRule{SourceCurrency: "CNY", TargetCurrency: "GBP", ExchangeRate: 9.1, MarkupMultiplier: 1.4, MinimumPrice: 10, RoundTo: 0.05, PriceEnding: 0.99}) {
		t.Fatalf("pricing = %+v, want profile pricing", settings.Pricing)
	}
}

func TestSheinSubmitPreferredWarehouseCodeUsesFirstCSVItem(t *testing.T) {
	t.Parallel()

	if got := sheinpub.SubmitPreferredWarehouseCode(sheinpub.SubmitPayloadSettings{WarehouseCode: "WH-CA-1,WH-US-1"}); got != "WH-CA-1" {
		t.Fatalf("preferred warehouse = %q, want WH-CA-1", got)
	}
}

func TestResolveSheinSubmitSettingsPrefersTaskSnapshotOverCurrentProfiles(t *testing.T) {
	t.Parallel()

	storeProfileRepo := newInMemoryStoreProfileRepository()
	svc := &service{
		adminDeps:      adminDependencies{storeProfileRepo: storeProfileRepo},
		submissionDeps: submissionDependencies{storeProfileRepo: storeProfileRepo},
		sheinSettings: SheinSettings{
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
		Pricing: sheinpub.PricingRule{
			SourceCurrency:   "CNY",
			TargetCurrency:   "USD",
			ExchangeRate:     7.2,
			MarkupMultiplier: 1.1,
			MinimumPrice:     9.99,
			RoundTo:          0.01,
		},
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}

	task := &Task{
		TenantID: "405",
		Request:  &GenerateRequest{},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:           902,
			Site:              "GB",
			WarehouseCode:     "WH-GB-7",
			DefaultStock:      66,
			DefaultSubmitMode: "save_draft",
			Pricing: sheinpub.PricingRule{
				SourceCurrency:   "CNY",
				TargetCurrency:   "GBP",
				ExchangeRate:     9.1,
				MarkupMultiplier: 1.4,
				MinimumPrice:     10,
				RoundTo:          0.05,
				PriceEnding:      0.99,
			},
			Strategy:         "country",
			Reason:           "snapshot persisted at task creation",
			MatchedProfileID: 12,
		},
	}
	settings := svc.resolveSheinSubmitSettings(ctx, task)
	storeID, err := svc.resolveSheinStoreID(ctx, task)
	if err != nil {
		t.Fatalf("resolveSheinStoreID error = %v", err)
	}
	if storeID != 902 {
		t.Fatalf("resolved store id = %d, want snapshot store 902", storeID)
	}
	if settings.Site != "GB" || settings.WarehouseCode != "WH-GB-7" || settings.DefaultStock != 66 || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("settings = %+v, want snapshot-backed settings", settings)
	}
	if settings.Pricing != (sheinpub.PricingRule{SourceCurrency: "CNY", TargetCurrency: "GBP", ExchangeRate: 9.1, MarkupMultiplier: 1.4, MinimumPrice: 10, RoundTo: 0.05, PriceEnding: 0.99}) {
		t.Fatalf("pricing = %+v, want snapshot pricing", settings.Pricing)
	}
}

func TestResolveSheinSubmitSettingsRehydratesProfileWhenSnapshotOnlyCarriesAccess(t *testing.T) {
	t.Parallel()

	storeProfileRepo := newInMemoryStoreProfileRepository()
	svc := &service{
		adminDeps:      adminDependencies{storeProfileRepo: storeProfileRepo},
		submissionDeps: submissionDependencies{storeProfileRepo: storeProfileRepo},
		sheinSettings: SheinSettings{
			Site:              "US",
			WarehouseCode:     "DEFAULT",
			DefaultStock:      100,
			DefaultSubmitMode: "publish",
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "406", UserID: "user-f"})
	_, err := svc.UpsertSheinStoreProfile(ctx, &ListingKitStoreProfile{
		StoreID:           904,
		Enabled:           true,
		Priority:          1,
		Site:              "GB",
		WarehouseCode:     "WH-GB-4",
		DefaultStock:      44,
		DefaultSubmitMode: "save_draft",
	})
	if err != nil {
		t.Fatalf("UpsertSheinStoreProfile error = %v", err)
	}

	task := &Task{
		TenantID: "406",
		Request: &GenerateRequest{
			SheinStoreID: 999,
		},
		SheinStoreResolutionSnapshot: &SheinStoreResolutionSnapshot{
			StoreID:           904,
			TenantAdminAccess: true,
		},
	}

	settings := svc.resolveSheinSubmitSettings(ctx, task)
	if settings.Site != "GB" || settings.WarehouseCode != "WH-GB-4" || settings.DefaultStock != 44 || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("settings = %+v, want repository-backed profile settings", settings)
	}
}

func TestApplySubmitSettingsProfileOverlaysProfileFields(t *testing.T) {
	t.Parallel()

	settings := applySubmitSettingsProfile(SheinSettings{
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
		Pricing: sheinpub.PricingRule{
			SourceCurrency:   "CNY",
			TargetCurrency:   "GBP",
			ExchangeRate:     9.1,
			MarkupMultiplier: 1.4,
			MinimumPrice:     10,
			RoundTo:          0.05,
			PriceEnding:      0.99,
		},
	})

	if settings.Site != "GB" || settings.WarehouseCode != "WH-GB-1" || settings.DefaultStock != 66 || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("settings = %+v, want profile-backed settings", settings)
	}
	if settings.Pricing != (sheinpub.PricingRule{SourceCurrency: "CNY", TargetCurrency: "GBP", ExchangeRate: 9.1, MarkupMultiplier: 1.4, MinimumPrice: 10, RoundTo: 0.05, PriceEnding: 0.99}) {
		t.Fatalf("pricing = %+v, want profile pricing", settings.Pricing)
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
