package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"gorm.io/gorm"

	orgresourceadapter "task-processor/internal/integration/orgresource"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	moneystore "task-processor/internal/integration/persistence/money"
	platformdatabase "task-processor/internal/platform/database"
)

func main() {
	manifest := flag.String("config", "", "absolute path to the private commercial schema-owner database JSON config")
	flag.Parse()
	if *manifest == "" {
		fmt.Fprintln(os.Stderr, "-config is required")
		os.Exit(2)
	}
	if err := run(*manifest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(manifestPath string) error {
	dbConfig, err := loadSchemaOwnerConfig(manifestPath)
	if err != nil {
		return err
	}
	db, err := platformdatabase.OpenExistingWritableContext(context.Background(), dbConfig)
	if err != nil {
		return fmt.Errorf("open commercial owner database: %w", err)
	}
	defer platformdatabase.Close(db)
	var canCreate bool
	if err := db.Raw("SELECT current_schema() = 'public' AND has_schema_privilege(current_user, 'public', 'CREATE')").Scan(&canCreate).Error; err != nil {
		return fmt.Errorf("verify commercial schema-owner privileges: %w", err)
	}
	if !canCreate {
		return fmt.Errorf("commercial schema migration role requires CREATE on the public schema")
	}
	if err := moneystore.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate money owner schema: %w", err)
	}
	if err := commercialstore.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate commercial billing schema: %w", err)
	}
	if err := orgresourceadapter.AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate organization resource schema: %w", err)
	}
	if err := grantCommercialRuntimeAccess(db); err != nil {
		return fmt.Errorf("grant commercial runtime table access: %w", err)
	}
	return nil
}

func grantCommercialRuntimeAccess(db interface{ Exec(string, ...any) *gorm.DB }) error {
	for _, grant := range commercialRuntimeGrants() {
		if err := db.Exec(grant).Error; err != nil {
			return err
		}
	}
	return nil
}

func commercialRuntimeGrants() []string {
	return []string{
		`GRANT USAGE ON SCHEMA public TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.ledger_organization_wallets, public.ledger_organization_wallet_reservations TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT ON TABLE public.ledger_organization_wallet_entries, public.ledger_organization_topup_settlements, public.ledger_organization_wallet_reversals TO commercial_owner_runtime`,
		`GRANT SELECT ON TABLE public.ledger_payment_settlements TO commercial_owner_runtime`,
		`GRANT SELECT ON TABLE public.commercial_offers TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT ON TABLE public.commercial_quotes, public.commercial_order_items TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.commercial_orders TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.saas_organization_resource_buckets, public.saas_organization_resource_operations, public.saas_organization_resource_reservations, public.saas_organization_resource_debts TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT ON TABLE public.saas_organization_resource_source_claims, public.saas_organization_resource_events, public.saas_organization_resource_audit_logs TO commercial_owner_runtime`,
		`GRANT USAGE, SELECT ON SEQUENCE public.saas_organization_resource_audit_logs_id_seq TO commercial_owner_runtime`,
	}
}

type schemaOwnerManifest struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	Password           string `json:"password"`
	Database           string `json:"database"`
	MaxConnections     int    `json:"maxConnections"`
	MaxIdleConnections int    `json:"maxIdleConnections"`
}

func loadSchemaOwnerConfig(path string) (*platformdatabase.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open private commercial schema-owner config: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest schemaOwnerManifest
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode commercial schema-owner config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("commercial schema-owner config must contain one JSON object")
	}
	if manifest.Host == "" || manifest.Port < 1 || manifest.Port > 65535 || manifest.User == "" || manifest.Password == "" || manifest.Database == "" || manifest.MaxConnections < 1 || manifest.MaxIdleConnections < 0 || manifest.MaxIdleConnections > manifest.MaxConnections {
		return nil, fmt.Errorf("commercial schema-owner config has invalid database settings")
	}
	if manifest.User == "commercial_runtime" || manifest.User == "commercial_owner_runtime" {
		return nil, fmt.Errorf("commercial runtime database roles cannot run schema migrations")
	}
	return &platformdatabase.Config{Host: manifest.Host, Port: manifest.Port, User: manifest.User, Password: manifest.Password, Database: manifest.Database, MaxConnections: manifest.MaxConnections, MaxIdleConnections: manifest.MaxIdleConnections}, nil
}
