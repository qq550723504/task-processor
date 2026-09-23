package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"ledger_organization_wallets",
		"ledger_organization_wallet_entries",
		"ledger_organization_wallet_reservations",
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
	} {
		if !strings.Contains(grants, "public."+table) {
			t.Errorf("runtime grants omit required owner table %s", table)
		}
	}
	if strings.Contains(grants, "ALL TABLES") || strings.Contains(grants, "GRANT ALL") {
		t.Fatal("commercial runtime grants must not widen to all tables")
	}
}
