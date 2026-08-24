package sourceaccount

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormRepositoryScopesByTenantAndIgnoresListingStore(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:source-account-test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatalf("AutoMigrateRepository() error = %v", err)
	}
	if err := db.Exec("CREATE TABLE listing_store (id INTEGER, tenant_id INTEGER, platform TEXT, status INTEGER, deleted INTEGER)").Error; err != nil {
		t.Fatalf("create listing_store: %v", err)
	}
	if err := db.Exec("INSERT INTO listing_store (id, tenant_id, platform, status, deleted) VALUES (42, 101, 'SHEIN', 0, 0)").Error; err != nil {
		t.Fatalf("insert listing_store: %v", err)
	}
	if err := db.Create(&sourceAccountRow{ID: 7, TenantID: 101, Platform: PlatformAlibaba1688, Label: "public fallback", ProfileRef: "profile-ref", Status: StatusEnabled, Deleted: 0}).Error; err != nil {
		t.Fatalf("insert source account: %v", err)
	}

	repository := NewGormRepository(db)
	account, err := repository.Get(context.Background(), 101, 7)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if account.ID != 7 || account.TenantID != 101 {
		t.Fatalf("account = %+v, want tenant-owned source account", account)
	}
	if _, err := repository.Get(context.Background(), 202, 7); ErrorCode(err) != SourceAccountUnavailable {
		t.Fatalf("cross-tenant Get() error code = %q, want %q", ErrorCode(err), SourceAccountUnavailable)
	}
	if _, err := repository.Get(context.Background(), 101, 42); ErrorCode(err) != SourceAccountUnavailable {
		t.Fatalf("listing_store ID lookup error code = %q, want %q", ErrorCode(err), SourceAccountUnavailable)
	}
}

func TestGormRepositoryRejectsDisabledAndDeletedAccounts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:source-account-disabled?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatalf("AutoMigrateRepository() error = %v", err)
	}
	if err := db.Create(&sourceAccountRow{ID: 8, TenantID: 101, Platform: PlatformAlibaba1688, ProfileRef: "disabled-ref", Status: StatusDisabled, Deleted: 0}).Error; err != nil {
		t.Fatalf("insert disabled account: %v", err)
	}
	if _, err := NewGormRepository(db).Get(context.Background(), 101, 8); ErrorCode(err) != SourceAccountDisabled {
		t.Fatalf("disabled Get() error code = %q, want %q", ErrorCode(err), SourceAccountDisabled)
	}
	if err := db.Create(&sourceAccountRow{ID: 9, TenantID: 101, Platform: PlatformAlibaba1688, ProfileRef: "deleted-ref", Status: StatusEnabled, Deleted: 1}).Error; err != nil {
		t.Fatalf("insert deleted account: %v", err)
	}
	if _, err := NewGormRepository(db).Get(context.Background(), 101, 9); ErrorCode(err) != SourceAccountUnavailable {
		t.Fatalf("deleted Get() error code = %q, want %q", ErrorCode(err), SourceAccountUnavailable)
	}
}
