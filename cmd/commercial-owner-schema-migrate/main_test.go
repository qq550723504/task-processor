package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/commercial/billing"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	"task-processor/internal/ledger/orgresource"
)

func TestFreshCommercialOwnerInstallsFiveFenDataRowOfferWithoutOverwritingIt(t *testing.T) {
	moneyDB := openOwnerSchemaTestDB(t, "money-price")
	commercialDB := openOwnerSchemaTestDB(t, "commercial-price")
	if err := migrateOwnerSchemas(moneyDB, commercialDB); err != nil {
		t.Fatal(err)
	}
	store, err := commercialstore.New(commercialDB)
	if err != nil {
		t.Fatal(err)
	}
	offer, err := store.ReadOffer(context.Background(), "data-row-1688-server-v1")
	if err != nil {
		t.Fatalf("fresh installation must expose the approved data-row offer: %v", err)
	}
	if offer.UnitPriceMinor != 5 || offer.Currency != "CNY" || offer.PricingVersion != "1688-server-5fen-v1" || offer.MinQuantity != 1 || offer.MaxQuantity < 1 {
		t.Fatalf("unexpected approved offer: %#v", offer)
	}
	quote, err := store.CreateQuote(context.Background(), billing.QuoteRequest{OrganizationID: "test-org", OfferID: offer.OfferID, Quantity: 3})
	if err != nil || quote.TotalMinor != 15 || quote.ResourceQuantity != 3 || quote.PricingVersion != offer.PricingVersion {
		t.Fatalf("data-row quote does not freeze 5 fen per row: %#v, %v", quote, err)
	}
	offer.UnitPriceMinor = 7 // A later owner-managed catalog change must survive schema initialization.
	offer.PricingVersion = "operator-next-version"
	if err := store.SaveOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	if err := migrateOwnerSchemas(moneyDB, commercialDB); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadOffer(context.Background(), offer.OfferID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnitPriceMinor != 7 || got.PricingVersion != "operator-next-version" {
		t.Fatalf("schema initialization overwrote an existing offer: %#v", got)
	}
}

func TestCommercialOwnerDoesNotCreateCompetingDataRowPrice(t *testing.T) {
	moneyDB := openOwnerSchemaTestDB(t, "money-existing-price")
	commercialDB := openOwnerSchemaTestDB(t, "commercial-existing-price")
	if err := commercialstore.AutoMigrate(commercialDB); err != nil {
		t.Fatal(err)
	}
	store, err := commercialstore.New(commercialDB)
	if err != nil {
		t.Fatal(err)
	}
	existing := billing.Offer{OfferID: "operator-data-row", ProductKind: billing.ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: billing.CurrencyCNY, UnitPriceMinor: 7, PricingVersion: "operator-v1", MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferActive}
	if err := store.CreateOffer(context.Background(), existing); err != nil {
		t.Fatal(err)
	}
	if err := migrateOwnerSchemas(moneyDB, commercialDB); err == nil {
		t.Fatal("a competing active DATA_ROW offer needs an explicit owner decision")
	}
	data, err := store.ListDataRowOffers(context.Background())
	if err != nil || len(data) != 1 || data[0].OfferID != existing.OfferID || data[0].UnitPriceMinor != 7 {
		t.Fatalf("existing offer changed after rejected initialization: %#v, %v", data, err)
	}
}

func TestCommercialOwnerRejectsScheduledCompetingDataRowPrice(t *testing.T) {
	moneyDB := openOwnerSchemaTestDB(t, "money-scheduled-price")
	commercialDB := openOwnerSchemaTestDB(t, "commercial-scheduled-price")
	if err := commercialstore.AutoMigrate(commercialDB); err != nil {
		t.Fatal(err)
	}
	store, err := commercialstore.New(commercialDB)
	if err != nil {
		t.Fatal(err)
	}
	startsAt := time.Now().UTC().Add(24 * time.Hour)
	existing := billing.Offer{OfferID: "scheduled-data-row", ProductKind: billing.ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: billing.CurrencyCNY, UnitPriceMinor: 7, PricingVersion: "operator-scheduled", MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferActive, StartsAt: &startsAt}
	if err := store.CreateOffer(context.Background(), existing); err != nil {
		t.Fatal(err)
	}
	if err := migrateOwnerSchemas(moneyDB, commercialDB); err == nil {
		t.Fatal("a scheduled active DATA_ROW offer must block a competing default price")
	}
	if _, err := store.ReadOffer(context.Background(), "data-row-1688-server-v1"); err == nil {
		t.Fatal("a competing default offer was inserted")
	}
}

func TestCommercialOwnerRejectsOccupiedDefaultOfferID(t *testing.T) {
	moneyDB := openOwnerSchemaTestDB(t, "money-occupied-price")
	commercialDB := openOwnerSchemaTestDB(t, "commercial-occupied-price")
	if err := commercialstore.AutoMigrate(commercialDB); err != nil {
		t.Fatal(err)
	}
	store, err := commercialstore.New(commercialDB)
	if err != nil {
		t.Fatal(err)
	}
	occupied := billing.Offer{OfferID: "data-row-1688-server-v1", ProductKind: billing.ProductAIPoint, ResourceType: orgresource.ResourceAIPoint, Currency: billing.CurrencyCNY, UnitPriceMinor: 9, PricingVersion: "unrelated", MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferActive}
	if err := store.CreateOffer(context.Background(), occupied); err != nil {
		t.Fatal(err)
	}
	if err := migrateOwnerSchemas(moneyDB, commercialDB); err == nil {
		t.Fatal("an offer ID owned by another product must not count as a DATA_ROW install")
	}
	got, err := store.ReadOffer(context.Background(), occupied.OfferID)
	if err != nil || got.ProductKind != billing.ProductAIPoint || got.UnitPriceMinor != occupied.UnitPriceMinor {
		t.Fatalf("occupied offer was changed: %#v, %v", got, err)
	}
}

func TestCommercialOwnerRejectsMalformedDefaultOffer(t *testing.T) {
	moneyDB := openOwnerSchemaTestDB(t, "money-malformed-price")
	commercialDB := openOwnerSchemaTestDB(t, "commercial-malformed-price")
	if err := commercialstore.AutoMigrate(commercialDB); err != nil {
		t.Fatal(err)
	}
	store, err := commercialstore.New(commercialDB)
	if err != nil {
		t.Fatal(err)
	}
	offer := billing.Offer{OfferID: "data-row-1688-server-v1", ProductKind: billing.ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: billing.CurrencyCNY, UnitPriceMinor: 5, PricingVersion: "approved", MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferActive}
	if err := store.CreateOffer(context.Background(), offer); err != nil {
		t.Fatal(err)
	}
	if err := commercialDB.Exec("UPDATE commercial_offers SET unit_price_minor = 0 WHERE offer_id = ?", offer.OfferID).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrateOwnerSchemas(moneyDB, commercialDB); err == nil {
		t.Fatal("a malformed zero-price DATA_ROW offer must not count as an approved installation")
	}
}

func TestLoadSchemaOwnerConfigUsesOwnerCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "schema-owner.json")
	contents := `{"host":"127.0.0.1","port":5434,"user":"postgres","password":"private","database":"commercial","maxConnections":2,"maxIdleConnections":1}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := loadSchemaOwnerConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.User != "postgres" || config.Password != "private" || config.Database != "commercial" {
		t.Fatalf("loaded schema owner config = %#v", config)
	}
}

func TestLoadSchemaOwnerConfigRejectsRuntimeRole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.json")
	contents := `{"host":"127.0.0.1","port":5434,"user":"commercial_owner_runtime","password":"private","database":"commercial","maxConnections":2,"maxIdleConnections":1}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSchemaOwnerConfig(path); err == nil {
		t.Fatal("runtime role must not be accepted for schema migration")
	}
}

func TestCommercialRuntimeGrantsStayWithinOwnedTables(t *testing.T) {
	grants := strings.Join(commercialRuntimeGrants(), "\n")
	if !strings.Contains(grants, "GRANT CONNECT ON DATABASE") {
		t.Fatal("current commercial owner role cannot connect after empty database bootstrap")
	}
	for _, table := range []string{
		"commercial_offers",
		"commercial_quotes",
		"commercial_orders",
		"commercial_order_items",
		"saas_organization_resource_buckets",
		"saas_organization_resource_operations",
		"saas_organization_resource_source_claims",
		"saas_organization_resource_events",
		"saas_organization_resource_reservations",
		"saas_organization_resource_debts",
		"saas_organization_resource_audit_logs",
		"saas_member_ai_point_limits",
		"saas_member_ai_point_months",
		"saas_member_resource_positions",
	} {
		if !strings.Contains(grants, "public."+table) {
			t.Errorf("runtime grants omit required owner table %s", table)
		}
	}
	if strings.Contains(grants, "saas_plans") || strings.Contains(grants, "saas_tenant_subscriptions") || strings.Contains(grants, "saas_tenant_entitlements") {
		t.Fatal("retired subscription permissions remain in current schema initialization")
	}
	if strings.Contains(grants, "ALL TABLES") || strings.Contains(grants, "GRANT ALL") {
		t.Fatal("commercial runtime grants must not widen to all tables")
	}
}

func TestMoneyRuntimeGrantsStayWithCanonicalMoneyOwner(t *testing.T) {
	grants := strings.Join(moneyRuntimeGrants(), "\n")
	for _, table := range []string{
		"ledger_payment_settlements",
		"ledger_organization_wallets",
		"ledger_organization_wallet_entries",
		"ledger_organization_wallet_reservations",
		"ledger_organization_wallet_reserve_decisions",
		"ledger_organization_topup_settlements",
		"ledger_organization_wallet_reversals",
	} {
		if !strings.Contains(grants, "public."+table) {
			t.Errorf("canonical money runtime grants omit required table %s", table)
		}
	}
	if strings.Contains(grants, "commercial_owner_runtime") || strings.Contains(grants, "ALL TABLES") || strings.Contains(grants, "GRANT ALL") {
		t.Fatal("money grants must stay scoped to referral_runtime and owned tables")
	}
}

func TestOwnerSchemaMigrationsKeepMoneyFactsAndWalletTogether(t *testing.T) {
	moneyDB := openOwnerSchemaTestDB(t, "money-owner")
	commercialDB := openOwnerSchemaTestDB(t, "commercial-owner")
	if err := migrateOwnerSchemas(moneyDB, commercialDB); err != nil {
		t.Fatalf("migrate owner schemas: %v", err)
	}

	for _, check := range []struct {
		db        *gorm.DB
		table     string
		wantExist bool
	}{
		{moneyDB, "ledger_payment_settlements", true},
		{moneyDB, "ledger_organization_wallets", true},
		{commercialDB, "commercial_orders", true},
		{commercialDB, "saas_organization_resource_buckets", true},
		{commercialDB, "saas_member_ai_point_limits", true},
		{commercialDB, "saas_plans", false},
		{commercialDB, "saas_tenant_entitlements", false},
		{commercialDB, "saas_purchased_plan_activations", false},
		{commercialDB, "saas_subscription_activation_fences", false},
		{moneyDB, "ledger_organization_wallet_reserve_decisions", true},
		{commercialDB, "ledger_payment_settlements", false},
		{commercialDB, "ledger_organization_wallets", false},
	} {
		if got := check.db.Migrator().HasTable(check.table); got != check.wantExist {
			t.Errorf("table %s exists = %t, want %t", check.table, got, check.wantExist)
		}
	}
}

func openOwnerSchemaTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+name+"-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}
