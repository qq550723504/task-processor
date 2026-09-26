package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

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
		"saas_plans",
		"saas_plan_modules",
		"saas_tenant_subscriptions",
		"saas_tenant_entitlements",
		"saas_subscription_activation_fences",
		"saas_purchased_plan_activations",
		"saas_subscription_audit_logs",
	} {
		if !strings.Contains(grants, "public."+table) {
			t.Errorf("runtime grants omit required owner table %s", table)
		}
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
		{commercialDB, "saas_purchased_plan_activations", true},
		{commercialDB, "saas_subscription_activation_fences", true},
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
