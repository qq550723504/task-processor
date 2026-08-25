package bootstrap

import (
	"context"
	"testing"

	"task-processor/internal/core/config"
	"task-processor/internal/tenantbridge"
)

func TestLegacyTenantResolverDatabaseConfigsUsesZitadelCandidates(t *testing.T) {
	cfg := &config.Config{Database: &config.DatabaseConfig{Host: "db", Database: "ignored"}}
	configs := LegacyTenantResolverDatabaseConfigs(cfg)
	if len(configs) != 2 {
		t.Fatalf("database configs = %d, want 2", len(configs))
	}
	if configs[0].Database != "zitadel_auth" || configs[1].Database != "zitadel" {
		t.Fatalf("database candidates = %#v, want zitadel_auth then zitadel", configs)
	}
}

func TestLegacyTenantMetadataTableExistsRejectsNilDatabase(t *testing.T) {
	if LegacyTenantMetadataTableExists(nil) {
		t.Fatal("LegacyTenantMetadataTableExists(nil) = true, want false")
	}
}

func TestConfigureFromConfigWithoutDatabaseClearsResolver(t *testing.T) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(staticResolver{})
	defer restore()

	closer, err := ConfigureFromConfig(&config.Config{}, nil)
	if err != nil {
		t.Fatalf("ConfigureFromConfig() error = %v", err)
	}
	if closer != nil {
		t.Fatal("ConfigureFromConfig() closer != nil without database")
	}
	if _, err := tenantbridge.ResolveLegacyTenantID(context.Background(), "zitadel-tenant"); err == nil {
		t.Fatal("ResolveLegacyTenantID() error = nil after resolver was cleared")
	}
}

type staticResolver struct{}

func (staticResolver) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	return 227, true, nil
}
