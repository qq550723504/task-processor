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
	"task-processor/internal/listingsubscription"
	platformdatabase "task-processor/internal/platform/database"
)

func main() {
	manifest := flag.String("config", "", "absolute path to the private commercial schema-owner database JSON config")
	moneyManifest := flag.String("money-config", "", "absolute path to the private canonical money schema-owner database JSON config")
	isolatedTrialCatalog := flag.Bool("isolated-trial-catalog", false, "explicit one-time trial catalog provisioning mode; never a runtime seed")
	expectedDatabase := flag.String("expected-database", "", "exact isolated trial database name")
	confirm := flag.String("confirm", "", "must be ISOLATED_TRIAL_ONLY for trial catalog mode")
	flag.Parse()
	if *isolatedTrialCatalog {
		if *manifest == "" || *moneyManifest != "" || *expectedDatabase == "" || *confirm != "ISOLATED_TRIAL_ONLY" {
			fmt.Fprintln(os.Stderr, "trial catalog mode requires -config, -expected-database and -confirm ISOLATED_TRIAL_ONLY; omit -money-config")
			os.Exit(2)
		}
		if err := runIsolatedTrial(*manifest, *expectedDatabase); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("isolated subscription catalog provisioned; start the application only after reviewing this database")
		return
	}
	if *expectedDatabase != "" || *confirm != "" {
		fmt.Fprintln(os.Stderr, "-expected-database and -confirm require -isolated-trial-catalog")
		os.Exit(2)
	}
	if *manifest == "" || *moneyManifest == "" {
		fmt.Fprintln(os.Stderr, "-config and -money-config are required")
		os.Exit(2)
	}
	if err := run(*manifest, *moneyManifest); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(manifestPath, moneyManifestPath string) error {
	dbConfig, err := loadSchemaOwnerConfig(manifestPath)
	if err != nil {
		return err
	}
	db, err := platformdatabase.OpenExistingWritableContext(context.Background(), dbConfig)
	if err != nil {
		return fmt.Errorf("open commercial owner database: %w", err)
	}
	defer platformdatabase.Close(db)
	moneyConfig, err := loadSchemaOwnerConfig(moneyManifestPath)
	if err != nil {
		return err
	}
	moneyDB, err := platformdatabase.OpenExistingWritableContext(context.Background(), moneyConfig)
	if err != nil {
		return fmt.Errorf("open canonical money owner database: %w", err)
	}
	defer platformdatabase.Close(moneyDB)
	for _, owner := range []struct {
		name string
		db   *gorm.DB
	}{{"commercial", db}, {"money", moneyDB}} {
		var canCreate bool
		if err := owner.db.Raw("SELECT current_schema() = 'public' AND has_schema_privilege(current_user, 'public', 'CREATE')").Scan(&canCreate).Error; err != nil {
			return fmt.Errorf("verify %s schema-owner privileges: %w", owner.name, err)
		}
		if !canCreate {
			return fmt.Errorf("%s schema migration role requires CREATE on the public schema", owner.name)
		}
	}
	if err := migrateOwnerSchemas(moneyDB, db); err != nil {
		return err
	}
	if err := grantMoneyRuntimeAccess(moneyDB); err != nil {
		return fmt.Errorf("grant canonical money runtime table access: %w", err)
	}
	if err := grantCommercialRuntimeAccess(db); err != nil {
		return fmt.Errorf("grant commercial runtime table access: %w", err)
	}
	return nil
}

func migrateOwnerSchemas(moneyDB, commercialDB *gorm.DB) error {
	if err := moneystore.AutoMigrate(moneyDB); err != nil {
		return fmt.Errorf("migrate canonical money owner schema: %w", err)
	}
	if err := commercialstore.AutoMigrate(commercialDB); err != nil {
		return fmt.Errorf("migrate commercial billing schema: %w", err)
	}
	if err := orgresourceadapter.AutoMigrate(commercialDB); err != nil {
		return fmt.Errorf("migrate organization resource schema: %w", err)
	}
	if err := listingsubscription.AutoMigrateRepository(commercialDB); err != nil {
		return fmt.Errorf("migrate subscription owner schema: %w", err)
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
		`GRANT SELECT ON TABLE public.commercial_offers TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT ON TABLE public.commercial_quotes, public.commercial_order_items TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.commercial_orders TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.saas_organization_resource_buckets, public.saas_organization_resource_operations, public.saas_organization_resource_reservations, public.saas_organization_resource_debts TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.saas_member_ai_point_limits, public.saas_member_ai_point_months TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT ON TABLE public.saas_organization_resource_source_claims, public.saas_organization_resource_events, public.saas_organization_resource_audit_logs TO commercial_owner_runtime`,
		`GRANT USAGE, SELECT ON SEQUENCE public.saas_organization_resource_audit_logs_id_seq TO commercial_owner_runtime`,
		`GRANT SELECT ON TABLE public.saas_plans, public.saas_plan_modules TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.saas_tenant_subscriptions, public.saas_subscription_activation_fences TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.saas_tenant_entitlements TO commercial_owner_runtime`,
		`GRANT SELECT, INSERT ON TABLE public.saas_purchased_plan_activations, public.saas_subscription_audit_logs TO commercial_owner_runtime`,
		`GRANT USAGE, SELECT ON SEQUENCE public.saas_tenant_subscriptions_id_seq, public.saas_tenant_entitlements_id_seq, public.saas_subscription_audit_logs_id_seq TO commercial_owner_runtime`,
	}
}

func moneyRuntimeGrants() []string {
	return []string{
		`GRANT USAGE ON SCHEMA public TO referral_runtime`,
		`GRANT SELECT, INSERT, UPDATE ON TABLE public.ledger_organization_wallets, public.ledger_organization_wallet_reservations TO referral_runtime`,
		`GRANT SELECT, INSERT ON TABLE public.ledger_organization_wallet_entries, public.ledger_organization_wallet_reserve_decisions, public.ledger_organization_topup_settlements, public.ledger_organization_wallet_reversals TO referral_runtime`,
		`GRANT SELECT ON TABLE public.ledger_payment_settlements TO referral_runtime`,
	}
}

func grantMoneyRuntimeAccess(db interface{ Exec(string, ...any) *gorm.DB }) error {
	for _, grant := range moneyRuntimeGrants() {
		if err := db.Exec(grant).Error; err != nil {
			return err
		}
	}
	return nil
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
