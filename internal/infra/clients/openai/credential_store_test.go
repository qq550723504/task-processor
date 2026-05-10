package openai

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "modernc.org/sqlite"
)

func TestGormCredentialResolverPrefersUserThenTenantConfig(t *testing.T) {
	db := openTestCredentialDB(t)
	resolver := NewGormCredentialResolver(db)
	fallback := testClientConfig("global-key", "global-model", "https://global.example.test/v1")

	if err := resolver.SaveCredential(context.Background(), AIClientCredential{
		TenantID:      "tenant-a",
		ClientName:    "default",
		APIKey:        "tenant-key",
		BaseURL:       "https://tenant.example.test/v1",
		Model:         "tenant-model",
		TimeoutSecond: 45,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("save tenant credential: %v", err)
	}
	if err := resolver.SaveCredential(context.Background(), AIClientCredential{
		TenantID:      "tenant-a",
		UserID:        "user-a",
		ClientName:    "default",
		APIKey:        "user-key",
		BaseURL:       "https://user.example.test/v1",
		Model:         "user-model",
		TimeoutSecond: 60,
		Enabled:       true,
	}); err != nil {
		t.Fatalf("save user credential: %v", err)
	}

	userResolved, err := resolver.ResolveClientConfig(WithIdentity(context.Background(), Identity{TenantID: "tenant-a", UserID: "user-a"}), "default", fallback)
	if err != nil {
		t.Fatalf("resolve user: %v", err)
	}
	if userResolved == nil || userResolved.Config.APIKey != "user-key" || userResolved.Config.BaseURL != "https://user.example.test/v1" || userResolved.Config.Model != "user-model" || userResolved.Config.Timeout != time.Minute {
		t.Fatalf("user resolved = %#v", userResolved)
	}

	tenantResolved, err := resolver.ResolveClientConfig(WithTenantID(context.Background(), "tenant-a"), "default", fallback)
	if err != nil {
		t.Fatalf("resolve tenant: %v", err)
	}
	if tenantResolved == nil || tenantResolved.Config.APIKey != "tenant-key" || tenantResolved.Config.BaseURL != "https://tenant.example.test/v1" || tenantResolved.Config.Model != "tenant-model" || tenantResolved.Config.Timeout != 45*time.Second {
		t.Fatalf("tenant resolved = %#v", tenantResolved)
	}

	missing, err := resolver.ResolveClientConfig(WithTenantID(context.Background(), "tenant-b"), "default", fallback)
	if err != nil {
		t.Fatalf("resolve missing: %v", err)
	}
	if missing != nil {
		t.Fatalf("missing resolved = %#v, want nil", missing)
	}
}

func openTestCredentialDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&AIClientCredential{}); err != nil {
		t.Fatalf("migrate credentials: %v", err)
	}
	return db
}
