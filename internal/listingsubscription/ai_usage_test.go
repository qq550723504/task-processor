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
	if err := repo.ReserveAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", 100, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SettleAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", 101, start.Add(2*time.Hour)); !errors.Is(err, ErrUsageQuotaExceeded) {
		t.Fatalf("settlement beyond reservation=%v, want quota exceeded", err)
	}
	if err := repo.ReserveAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-2", 1, start.Add(time.Hour)); !errors.Is(err, ErrUsageQuotaExceeded) {
		t.Fatalf("second reservation=%v, want quota exceeded", err)
	}
	if _, err := repo.SettleAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-1", 12, start.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveAIInvocationUsage(context.Background(), "org-1", "member-1", "inv-2", 88, start.Add(3*time.Hour)); err != nil {
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

func TestAIInvocationReservationRejectsChangedMaximum(t *testing.T) {
	db, repo, start, _ := newAIUsageTestRepository(t)
	ctx := context.Background()
	if err := repo.ReserveAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 30, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 31, start.Add(time.Hour)); !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("changed reservation maximum=%v, want identity conflict", err)
	}
	var reservation usageEventRow
	if err := db.Where("source_type = ? AND source_id = ?", "ai_invocation_reservation", "inv-1").Take(&reservation).Error; err != nil {
		t.Fatal(err)
	}
	if reservation.Quantity != 30 || reservation.Status != string(UsageEventReserved) {
		t.Fatalf("reservation changed: %+v", reservation)
	}
	if _, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 12, start.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 31, start.Add(3*time.Hour)); !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("changed maximum after settlement=%v, want identity conflict", err)
	}
}

func TestUnreservedAIInvocationRetainsCurrentWindowReplayContract(t *testing.T) {
	db, repo, start, end := newAIUsageTestRepository(t)
	ctx := context.Background()
	if _, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "legacy-inv-1", 12, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	nextStart, nextEnd := start.Add(12*time.Hour), end.Add(12*time.Hour)
	if err := db.Exec("UPDATE saas_tenant_entitlements SET starts_at = ?, expires_at = ? WHERE tenant_id = ? AND module_code = ?", nextStart, nextEnd, "org-1", ModuleListingKit).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "legacy-inv-1", 12, end.Add(time.Hour)); !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("unreserved cross-window replay=%v, want original identity conflict", err)
	}
}

func TestAIInvocationSettlementUsesOriginalReservationWindowAfterRollover(t *testing.T) {
	db, repo, start, end := newAIUsageTestRepository(t)
	ctx := context.Background()
	if err := repo.ReserveAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 30, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	previousPeriod := UsagePeriodKeyForWindow(start, end)
	nextStart, nextEnd := start.Add(12*time.Hour), end.Add(12*time.Hour)
	if err := db.Exec("UPDATE saas_tenant_entitlements SET starts_at = ?, expires_at = ? WHERE tenant_id = ? AND module_code = ?", nextStart, nextEnd, "org-1", ModuleListingKit).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE account_member_token_allocations SET window_start = ?, window_end = ? WHERE organization_id = ? AND member_id = ?", nextStart, nextEnd, "org-1", "member-1").Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.ReserveAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 30, start.Add(time.Hour)); err != nil {
		t.Fatalf("exact replay of original reservation after rollover: %v", err)
	}
	first, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 12, end.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 12, end.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if first.EventID != second.EventID || first.PeriodKey != previousPeriod || first.Quantity != 12 || first.Status != UsageEventCommitted {
		t.Fatalf("settlement first=%+v second=%+v, want one original-window commit", first, second)
	}
	if _, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 13, end.Add(time.Hour)); !errors.Is(err, ErrUsageDuplicateIdentity) {
		t.Fatalf("changed observed usage=%v, want identity conflict", err)
	}
	var original usageBucketRow
	if err := db.Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", "org-1", ModuleListingKit, previousPeriod, usageMetricAITokens).Take(&original).Error; err != nil {
		t.Fatal(err)
	}
	if original.Reserved != 0 || original.Committed != 12 {
		t.Fatalf("original bucket=%+v", original)
	}
	var newEvents int64
	if err := db.Model(&usageEventRow{}).Where("tenant_id = ? AND metric = ? AND period_key = ?", "org-1", usageMetricAITokens, UsagePeriodKeyForWindow(nextStart, nextEnd)).Count(&newEvents).Error; err != nil {
		t.Fatal(err)
	}
	if newEvents != 0 {
		t.Fatalf("new-window usage events=%d, want zero", newEvents)
	}
}

func TestAIInvocationSettlementFailureKeepsOriginalReservation(t *testing.T) {
	db, repo, start, end := newAIUsageTestRepository(t)
	ctx := context.Background()
	if err := repo.ReserveAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 30, start.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TRIGGER fail_ai_observation BEFORE INSERT ON saas_usage_events WHEN NEW.source_type = 'ai_invocation' BEGIN SELECT RAISE(ABORT, 'forced observation write failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 12, start.Add(2*time.Hour)); err == nil {
		t.Fatal("expected observation write failure")
	}
	var reservation usageEventRow
	if err := db.Where("source_type = ? AND source_id = ?", "ai_invocation_reservation", "inv-1").Take(&reservation).Error; err != nil {
		t.Fatal(err)
	}
	if reservation.Status != string(UsageEventReserved) {
		t.Fatalf("reservation status=%s, want reserved", reservation.Status)
	}
	var bucket usageBucketRow
	if err := db.Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", "org-1", ModuleListingKit, UsagePeriodKeyForWindow(start, end), usageMetricAITokens).Take(&bucket).Error; err != nil {
		t.Fatal(err)
	}
	if bucket.Reserved != 30 || bucket.Committed != 0 {
		t.Fatalf("bucket after rolled-back settlement=%+v", bucket)
	}
	if err := db.Exec("DROP TRIGGER fail_ai_observation").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SettleAIInvocationUsage(ctx, "org-1", "member-1", "inv-1", 12, start.Add(2*time.Hour)); err != nil {
		t.Fatalf("recovery settlement: %v", err)
	}
}

func newAIUsageTestRepository(t *testing.T) (*gorm.DB, *GormRepository, time.Time, time.Time) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ai-owner-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateRepository(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE account_member_token_locks (organization_id TEXT PRIMARY KEY, updated_at DATETIME NOT NULL); CREATE TABLE account_member_token_allocations (organization_id TEXT NOT NULL, member_id TEXT NOT NULL, metric TEXT NOT NULL, allocated INTEGER NOT NULL, version INTEGER NOT NULL, active BOOLEAN NOT NULL, window_start DATETIME NOT NULL, window_end DATETIME NOT NULL, updated_at DATETIME NOT NULL, PRIMARY KEY (organization_id,member_id,metric))`).Error; err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	end := start.Add(48 * time.Hour)
	if err := db.Exec(`INSERT INTO saas_tenant_entitlements (tenant_id,module_code,status,starts_at,expires_at,limits) VALUES (?,?,?,?,?,?)`, "org-1", ModuleListingKit, StatusActive, start, end, `{"ai_tokens":100}`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO account_member_token_allocations (organization_id,member_id,metric,allocated,version,active,window_start,window_end,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`, "org-1", "member-1", "token", 100, 1, true, start, end, start).Error; err != nil {
		t.Fatal(err)
	}
	return db, NewGormRepository(db), start, end
}
