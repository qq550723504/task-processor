package tenantbridge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type failingResolver struct{}

func (failingResolver) ResolveLegacyTenantID(context.Context, string) (int64, bool, error) {
	return 0, false, errors.New("metadata table is unavailable")
}

func TestResolveLegacyTenantIDFallsBackToNumericTenantID(t *testing.T) {
	t.Parallel()

	restore := ConfigureLegacyTenantResolver(nil)
	t.Cleanup(restore)

	value, err := ResolveLegacyTenantID(context.Background(), "246")
	if err != nil {
		t.Fatalf("ResolveLegacyTenantID error = %v", err)
	}
	if value != 246 {
		t.Fatalf("tenant id = %d, want 246", value)
	}
}

func TestResolveLegacyTenantIDUsesMetadataMapping(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("org_metadata2").AutoMigrate(&metadataRow{}); err != nil {
		t.Fatalf("migrate org_metadata2: %v", err)
	}
	if err := db.Table("org_metadata2").Create(map[string]any{
		"org_id":        "373211199677923496",
		"sequence":      1,
		"key":           "yudao_tenant_id",
		"value":         []byte("227"),
		"owner_removed": false,
	}).Error; err != nil {
		t.Fatalf("seed org_metadata2: %v", err)
	}

	restore := ConfigureLegacyTenantResolver(NewMetadataResolver(db, WithTableName("org_metadata2")))
	t.Cleanup(restore)

	value, err := ResolveLegacyTenantID(context.Background(), "373211199677923496")
	if err != nil {
		t.Fatalf("ResolveLegacyTenantID error = %v", err)
	}
	if value != 227 {
		t.Fatalf("tenant id = %d, want 227", value)
	}
}

func TestResolveLegacyTenantIDFallsBackToNumericTenantIDWhenResolverErrors(t *testing.T) {
	t.Parallel()

	restore := ConfigureLegacyTenantResolver(failingResolver{})
	t.Cleanup(restore)

	value, err := ResolveLegacyTenantID(context.Background(), "1")
	if err != nil {
		t.Fatalf("ResolveLegacyTenantID error = %v", err)
	}
	if value != 1 {
		t.Fatalf("tenant id = %d, want 1", value)
	}
}

func TestMetadataResolverResolvesOrganizationIDFromLegacyTenantMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("org_metadata2").AutoMigrate(&metadataRow{}); err != nil {
		t.Fatalf("migrate org_metadata2: %v", err)
	}
	if err := db.Table("org_metadata2").Create(map[string]any{
		"org_id":        "org-X",
		"sequence":      1,
		"key":           "yudao_tenant_id",
		"value":         []byte("101"),
		"owner_removed": false,
	}).Error; err != nil {
		t.Fatalf("seed org_metadata2: %v", err)
	}

	resolver := NewMetadataResolver(db, WithTableName("org_metadata2"))
	organizationID, ok, err := resolver.ResolveOrganizationID(context.Background(), 101)
	if err != nil {
		t.Fatalf("ResolveOrganizationID: %v", err)
	}
	if !ok || organizationID != "org-X" {
		t.Fatalf("organization = %q, %v; want org-X, true", organizationID, ok)
	}

	organizationID, ok, err = resolver.ResolveOrganizationID(context.Background(), 202)
	if err != nil || ok || organizationID != "" {
		t.Fatalf("unmapped organization = %q, %v, %v; want empty, false, nil", organizationID, ok, err)
	}
}

func TestMetadataResolverResolveOrganizationIDSanitizesDatabaseFailure(t *testing.T) {
	const sensitiveTable = "private_metadata_table"
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	_, _, err = NewMetadataResolver(db, WithTableName(sensitiveTable)).ResolveOrganizationID(context.Background(), 101)
	if err == nil {
		t.Fatal("ResolveOrganizationID error = nil, want database failure")
	}
	if !strings.Contains(err.Error(), "resolve organization id") {
		t.Fatalf("error = %q, want operation context", err)
	}
	for _, raw := range []string{sensitiveTable, "101", "no such table"} {
		if strings.Contains(err.Error(), raw) {
			t.Errorf("error contains sensitive database detail %q: %q", raw, err)
		}
	}
}

func TestMetadataResolverResolveOrganizationIDRejectsAmbiguousMapping(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("org_metadata2").AutoMigrate(&metadataRow{}); err != nil {
		t.Fatalf("migrate org_metadata2: %v", err)
	}
	for _, organizationID := range []string{"org-X", "org-Y"} {
		if err := db.Table("org_metadata2").Create(map[string]any{
			"org_id":        organizationID,
			"sequence":      1,
			"key":           "yudao_tenant_id",
			"value":         []byte("101"),
			"owner_removed": false,
		}).Error; err != nil {
			t.Fatalf("seed org_metadata2: %v", err)
		}
	}

	organizationID, ok, err := NewMetadataResolver(db, WithTableName("org_metadata2")).ResolveOrganizationID(context.Background(), 101)
	if err == nil || ok || organizationID != "" {
		t.Fatalf("ambiguous organization = %q, %v, %v; want empty, false, error", organizationID, ok, err)
	}
	for _, raw := range []string{"101", "org-X", "org-Y"} {
		if strings.Contains(err.Error(), raw) {
			t.Errorf("error contains ambiguous mapping detail %q: %q", raw, err)
		}
	}
}
