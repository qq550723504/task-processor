package listingkit

import (
	"context"
	"errors"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestSettingsAdminServiceGetSheinSettingsAttachesAvailableStores(t *testing.T) {
	t.Parallel()

	svc := &service{

		sheinSettings: SheinSettings{
			Site:              "US",
			WarehouseCode:     "WH-US-1",
			DefaultSubmitMode: "publish",
		}, sheinSharedDeps: sheinSharedDependencies{storeCatalog: &stubSheinStoreCatalog{
			options: []SheinStoreOption{
				{ID: 870, Name: "primary"},
				{ID: 871, Name: "backup"},
			},
		}},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "227", UserID: "user-settings"})

	settings, err := svc.GetSheinSettings(ctx)
	if err != nil {
		t.Fatalf("GetSheinSettings error = %v", err)
	}
	if len(settings.AvailableStores) != 2 {
		t.Fatalf("available stores = %+v, want 2 options", settings.AvailableStores)
	}
	if settings.AvailableStores[0].ID != 870 || settings.AvailableStores[1].ID != 871 {
		t.Fatalf("available stores = %+v, want catalog-backed options", settings.AvailableStores)
	}
}

func TestSettingsAdminServicePropagatesStoreCatalogFailure(t *testing.T) {
	t.Parallel()

	catalogErr := errors.New("store catalog unavailable")
	svc := &service{
		sheinSettings: SheinSettings{Site: "US"},
		sheinSharedDeps: sheinSharedDependencies{storeCatalog: &stubSheinStoreCatalog{
			err: catalogErr,
		}},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "227", UserID: "user-settings"})

	_, err := svc.GetSheinSettings(ctx)
	if !errors.Is(err, catalogErr) {
		t.Fatalf("GetSheinSettings error = %v, want catalog error", err)
	}
}

func TestSettingsAdminServiceHealthReadSkipsStoreCatalog(t *testing.T) {
	t.Parallel()

	svc := &service{
		sheinSettings: SheinSettings{Site: "US", DefaultSubmitMode: "save_draft"},
		sheinSharedDeps: sheinSharedDependencies{storeCatalog: &stubSheinStoreCatalog{
			err: errors.New("store catalog unavailable"),
		}},
	}

	settings, err := svc.GetSheinSettingsForHealth(context.Background())
	if err != nil {
		t.Fatalf("GetSheinSettingsForHealth error = %v", err)
	}
	if settings.Site != "US" || settings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("health settings = %+v, want current configuration", settings)
	}
	if settings.AvailableStores != nil {
		t.Fatalf("health settings available stores = %+v, want catalog-free result", settings.AvailableStores)
	}
}

func TestSettingsAdminServiceUpdateSheinSettingsNormalizesAndPersistsValues(t *testing.T) {
	t.Parallel()

	svc := &service{
		sheinSettings: SheinSettings{
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
	if svc.sheinSettings.Site != "GB" {
		t.Fatalf("persisted shein settings = %+v, want updated values", svc.sheinSettings)
	}
	if svc.sheinSettings.UpdatedAt == nil || svc.sheinSettings.UpdatedAt.IsZero() {
		t.Fatalf("persisted updated_at = %v, want non-zero", svc.sheinSettings.UpdatedAt)
	}
}

func TestSettingsAdminServiceUpdatePersistsBeforeReportingStoreCatalogFailure(t *testing.T) {
	t.Parallel()

	catalogErr := errors.New("store catalog unavailable")
	svc := &service{
		sheinSettings: SheinSettings{Site: "US"},
		sheinSharedDeps: sheinSharedDependencies{storeCatalog: &stubSheinStoreCatalog{
			err: catalogErr,
		}},
	}
	ctx := openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "227", UserID: "user-settings"})

	_, err := svc.UpdateSheinSettings(ctx, &SheinSettings{
		Site:              "GB",
		WarehouseCode:     "WH-GB-1",
		DefaultStock:      88,
		DefaultSubmitMode: "save_draft",
	})
	if !errors.Is(err, catalogErr) {
		t.Fatalf("UpdateSheinSettings error = %v, want catalog error", err)
	}
	if svc.sheinSettings.Site != "GB" || svc.sheinSettings.WarehouseCode != "WH-GB-1" || svc.sheinSettings.DefaultStock != 88 || svc.sheinSettings.DefaultSubmitMode != "save_draft" {
		t.Fatalf("persisted settings = %+v, want updated values despite catalog failure", svc.sheinSettings)
	}
	if svc.sheinSettings.UpdatedAt == nil || svc.sheinSettings.UpdatedAt.IsZero() {
		t.Fatalf("persisted updated_at = %v, want non-zero", svc.sheinSettings.UpdatedAt)
	}
}
