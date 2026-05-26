package listingkit

import (
	"context"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestSettingsAdminServiceGetSheinSettingsAttachesAvailableStores(t *testing.T) {
	t.Parallel()

	svc := &service{
		sheinStoreCatalog: &stubSheinStoreCatalog{
			options: []SheinStoreOption{
				{ID: 870, Name: "primary"},
				{ID: 871, Name: "backup"},
			},
		},
		sheinSettings: SheinSettings{
			DefaultStoreID:    870,
			Site:              "US",
			WarehouseCode:     "WH-US-1",
			DefaultSubmitMode: "publish",
		},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "227", UserID: "user-settings"})

	settings, err := svc.GetSheinSettings(ctx)
	if err != nil {
		t.Fatalf("GetSheinSettings error = %v", err)
	}
	if settings.DefaultStoreID != 870 {
		t.Fatalf("default store id = %d, want 870", settings.DefaultStoreID)
	}
	if len(settings.AvailableStores) != 2 {
		t.Fatalf("available stores = %+v, want 2 options", settings.AvailableStores)
	}
	if settings.AvailableStores[0].ID != 870 || settings.AvailableStores[1].ID != 871 {
		t.Fatalf("available stores = %+v, want catalog-backed options", settings.AvailableStores)
	}
}

func TestSettingsAdminServiceUpdateSheinSettingsNormalizesAndPersistsValues(t *testing.T) {
	t.Parallel()

	svc := &service{
		sheinSettings: SheinSettings{
			DefaultStoreID:    869,
			Site:              "US",
			WarehouseCode:     "WH-US-1",
			DefaultStock:      50,
			DefaultSubmitMode: "publish",
			Pricing: sheinpub.PricingRule{
				SourceCurrency:   "CNY",
				TargetCurrency:   "USD",
				ExchangeRate:     7.2,
				MarkupMultiplier: 2,
				MinimumPrice:     9.99,
				RoundTo:          0.01,
			},
		},
	}

	settings, err := svc.UpdateSheinSettings(context.Background(), &SheinSettings{
		DefaultStoreID:    900,
		Site:              "gb",
		WarehouseCode:     "WH-GB-1",
		DefaultStock:      88,
		DefaultSubmitMode: "save_draft",
		Pricing: sheinpub.PricingRule{
			TargetCurrency:   "eur",
			ExchangeRate:     8.1,
			MarkupMultiplier: 2.5,
			MinimumPrice:     12.34,
			RoundTo:          0.05,
		},
	})
	if err != nil {
		t.Fatalf("UpdateSheinSettings error = %v", err)
	}
	if settings.DefaultStoreID != 900 {
		t.Fatalf("default store id = %d, want 900", settings.DefaultStoreID)
	}
	if settings.Site != "GB" {
		t.Fatalf("site = %q, want GB", settings.Site)
	}
	if settings.WarehouseCode != "WH-GB-1" {
		t.Fatalf("warehouse code = %q, want WH-GB-1", settings.WarehouseCode)
	}
	if settings.DefaultStock != 88 {
		t.Fatalf("default stock = %d, want 88", settings.DefaultStock)
	}
	if settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("submit mode = %q, want save_draft", settings.DefaultSubmitMode)
	}
	if settings.Pricing.TargetCurrency != "EUR" {
		t.Fatalf("pricing target currency = %q, want EUR", settings.Pricing.TargetCurrency)
	}
	if svc.sheinSettings.Site != "GB" || svc.sheinSettings.DefaultStoreID != 900 {
		t.Fatalf("persisted shein settings = %+v, want updated values", svc.sheinSettings)
	}
	if svc.sheinSettings.UpdatedAt == nil || svc.sheinSettings.UpdatedAt.IsZero() {
		t.Fatalf("persisted updated_at = %v, want non-zero", svc.sheinSettings.UpdatedAt)
	}
}
