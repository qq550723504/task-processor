package tenantbridge

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

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
