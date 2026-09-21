package listingsubscription

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSettleAIInvocationUsageIsIdempotentAndWindowBound(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai-usage-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE account_member_token_locks (organization_id TEXT PRIMARY KEY, updated_at DATETIME NOT NULL); CREATE TABLE account_member_token_allocations (organization_id TEXT NOT NULL, member_id TEXT NOT NULL, metric TEXT NOT NULL, allocated INTEGER NOT NULL, version INTEGER NOT NULL, active BOOLEAN NOT NULL, window_start DATETIME NOT NULL, window_end DATETIME NOT NULL, updated_at DATETIME NOT NULL, PRIMARY KEY (organization_id,member_id,metric))`).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO saas_tenant_entitlements (tenant_id,module_code,status,starts_at,expires_at,limits) VALUES (?,?,?,?,?,?)`, "org-1", ModuleListingKit, StatusActive, start, end, `{"ai_tokens":100}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO account_member_token_allocations (organization_id,member_id,metric,allocated,version,active,window_start,window_end,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`, "org-1", "member-1", "token", 100, 1, true, start, end, start).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewGormRepository(db)
	first, err := repo.SettleAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", 12, start.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.SettleAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", 12, start.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if first.EventID != second.EventID || second.Status != UsageEventCommitted {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	if _, err := repo.SettleAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", 13, start.Add(2*time.Hour)); !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("quantity conflict=%v", err)
	}
	var count int64
	if err := db.Model(&usageEventRow{}).Where("source_type = ? AND source_id = ?", "ai_invocation", "inv-1").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("events=%d", count)
	}
}

func TestAIInvocationReservationPrecedesProviderAndReleasesToObservedUsage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai-reservation-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE account_member_token_locks (organization_id TEXT PRIMARY KEY, updated_at DATETIME NOT NULL); CREATE TABLE account_member_token_allocations (organization_id TEXT NOT NULL, member_id TEXT NOT NULL, metric TEXT NOT NULL, allocated INTEGER NOT NULL, version INTEGER NOT NULL, active BOOLEAN NOT NULL, window_start DATETIME NOT NULL, window_end DATETIME NOT NULL, updated_at DATETIME NOT NULL, PRIMARY KEY (organization_id,member_id,metric))`).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Exec(`INSERT INTO saas_tenant_entitlements (tenant_id,module_code,status,starts_at,expires_at,limits) VALUES (?,?,?,?,?,?)`, "org-1", ModuleListingKit, StatusActive, start, end, `{"ai_tokens":200}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO account_member_token_allocations (organization_id,member_id,metric,allocated,version,active,window_start,window_end,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`, "org-1", "member-1", "token", 100, 1, true, start, end, start).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewGormRepository(db)
	if err := repo.ReserveAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-2", start.Add(time.Hour)); !errors.Is(err, ErrUsageQuotaExceeded) {
		t.Fatalf("second reservation=%v, want quota exceeded", err)
	}
	if _, err := repo.SettleAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", 12, start.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-2", start.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var bucket usageBucketRow
	if err := db.Where("tenant_id = ? AND metric = ?", "org-1", usageMetricAITokens).Take(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Committed != 12 || bucket.Reserved != 88 {
		t.Fatalf("bucket=%+v, want committed=12 reserved=88", bucket)
	}
}
