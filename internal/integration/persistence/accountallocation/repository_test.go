package accountallocation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	domain "task-processor/internal/accountallocation"
)

func TestCommittedAIUsageAuditReadSurvivesRepositoryReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commercial.db")
	first, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(first); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	row := commercialUsageEventRow{EventID: "event-reopen", TenantID: "org-1", ModuleCode: "listingkit", MemberID: "member-1", Metric: "ai_tokens", Quantity: 7, PeriodKey: "2026-09", SourceType: "ai_invocation", SourceID: "inv-1", IdempotencyKey: "event-reopen", Status: "committed", OccurredAt: now}
	if err := first.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	connection, err := first.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	reopenedConnection, err := reopened.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopenedConnection.Close() })
	repo, err := New(reopened)
	if err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListCommittedAIUsageAudit(context.Background(), "org-1", 20, nil)
	if err != nil || len(page.Items) != 1 || page.Items[0].Quantity != 7 || page.Items[0].MemberID != "member-1" || page.Items[0].SourceID != "inv-1" {
		t.Fatalf("reopened page=%+v err=%v", page, err)
	}
}

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:account-allocation-"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO saas_tenant_entitlements (tenant_id, module_code, status, starts_at, expires_at, limits) VALUES (?, ?, ?, ?, ?, ?)`, "org-1", "listingkit", "active", time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), `{"ai_tokens":100}`).Error; err != nil {
		t.Fatal(err)
	}
	repo, err := New(db)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func testQuota() domain.Quota {
	return domain.Quota{OrganizationID: "org-1", Metric: domain.MetricToken, Total: 100, WindowStart: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
}
func setInput(member, key string, target, version int64) domain.SetTargetInput {
	return domain.SetTargetInput{OrganizationID: "org-1", MemberID: member, Target: target, ExpectedVersion: version, IdempotencyKey: key, ActorID: "admin-1"}
}

func TestCommittedAIUsageAuditReadPaginatesAndScopesCanonicalEvents(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)
	for _, row := range []commercialUsageEventRow{
		{EventID: "event-a", TenantID: "org-1", MemberID: "member-1", Metric: "ai_tokens", Quantity: 7, SourceType: "ai_invocation", SourceID: "inv-a", Status: "committed", OccurredAt: now},
		{EventID: "event-b", TenantID: "org-1", MemberID: "member-2", Metric: "ai_tokens", Quantity: 8, SourceType: "ai_invocation", SourceID: "inv-b", Status: "committed", OccurredAt: now},
		{EventID: "event-c", TenantID: "org-1", MemberID: "member-3", Metric: "ai_tokens", Quantity: 9, SourceType: "ai_invocation", SourceID: "inv-c", Status: "committed", OccurredAt: now.Add(-time.Second)},
		{EventID: "foreign", TenantID: "org-2", MemberID: "member-x", Metric: "ai_tokens", Quantity: 10, SourceType: "ai_invocation", SourceID: "inv-x", Status: "committed", OccurredAt: now.Add(time.Second)},
		{EventID: "pending", TenantID: "org-1", MemberID: "member-1", Metric: "ai_tokens", Quantity: 11, SourceType: "ai_invocation", SourceID: "inv-p", Status: "reserved", OccurredAt: now.Add(time.Second)},
		{EventID: "other", TenantID: "org-1", MemberID: "member-1", Metric: "ai_tokens", Quantity: 12, SourceType: "manual", SourceID: "inv-m", Status: "committed", OccurredAt: now.Add(time.Second)},
	} {
		row.ModuleCode = "listingkit"
		row.PeriodKey = "2026-09"
		row.IdempotencyKey = row.EventID
		if err := repo.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.ListCommittedAIUsageAudit(ctx, "org-1", 2, nil)
	if err != nil || len(first.Items) != 2 || first.Items[0].EventID != "event-b" || first.Items[1].EventID != "event-a" || first.Next == nil {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	restarted, err := New(repo.db)
	if err != nil {
		t.Fatal(err)
	}
	second, err := restarted.ListCommittedAIUsageAudit(ctx, "org-1", 2, first.Next)
	if err != nil || len(second.Items) != 1 || second.Items[0].EventID != "event-c" || second.Next != nil {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	foreign, err := restarted.ListCommittedAIUsageAudit(ctx, "org-2", 2, nil)
	if err != nil || len(foreign.Items) != 1 || foreign.Items[0].EventID != "foreign" {
		t.Fatalf("foreign=%+v err=%v", foreign, err)
	}
}

func TestSetTargetIsVersionedIdempotentAndAudited(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	q := testQuota()
	first, err := repo.SetTarget(ctx, q, setInput("member-1", "op-1", 60, 0))
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || first.Allocated != 60 || !first.Active {
		t.Fatalf("first=%+v", first)
	}
	replay, err := repo.SetTarget(ctx, q, setInput("member-1", "op-1", 60, 0))
	if err != nil {
		t.Fatal(err)
	}
	if replay != first {
		t.Fatalf("replay=%+v first=%+v", replay, first)
	}
	_, err = repo.SetTarget(ctx, q, setInput("member-1", "op-1", 61, 0))
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("err=%v", err)
	}
	_, err = repo.SetTarget(ctx, q, setInput("member-1", "op-2", 50, 0))
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale err=%v", err)
	}
	var audits int64
	if err := repo.db.Model(&auditRow{}).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if audits != 1 {
		t.Fatalf("audits=%d", audits)
	}
}

func TestSetTargetEnforcesPoolAndConsumedFloorAndRevoke(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	q := testQuota()
	if _, err := repo.SetTarget(ctx, q, setInput("member-1", "op-1", 70, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetTarget(ctx, q, setInput("member-2", "op-2", 31, 0)); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("pool err=%v", err)
	}
	if err := repo.Consume(ctx, q, domain.ConsumeInput{OrganizationID: "org-1", MemberID: "member-1", Quantity: 20, IdempotencyKey: "use-1", SourceType: "test", SourceID: "call-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.SetTarget(ctx, q, setInput("member-2", "op-2b", 30, 0)); err != nil {
		t.Fatalf("active consumed quota should remain inside the member target pool: %v", err)
	}
	if _, err := repo.SetTarget(ctx, q, setInput("member-1", "op-3", 19, 1)); !errors.Is(err, domain.ErrConsumedFloor) {
		t.Fatalf("floor err=%v", err)
	}
	revoked, err := repo.SetTarget(ctx, q, setInput("member-1", "op-4", 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Active || revoked.Allocated != 20 || revoked.Consumed != 20 || revoked.Remaining != 0 {
		t.Fatalf("revoked=%+v", revoked)
	}
	if err := repo.Consume(ctx, q, domain.ConsumeInput{OrganizationID: "org-1", MemberID: "member-1", Quantity: 1, IdempotencyKey: "use-2", SourceType: "test", SourceID: "call-2"}); !errors.Is(err, domain.ErrAllocationRequired) {
		t.Fatalf("revoke consume err=%v", err)
	}
	page, err := repo.Snapshot(ctx, q)
	if err != nil {
		t.Fatal(err)
	}
	if page.Enterprise.Allocated != 30 || page.Enterprise.Unallocated != 50 || page.Enterprise.Consumed != 20 {
		t.Fatalf("page=%+v", page)
	}
}

func TestSetTargetIdempotencyKeyRejectsCrossWindowReplay(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	firstWindow := testQuota()
	input := setInput("member-1", "allocation-1", 10, 0)
	if _, err := repo.SetTarget(ctx, firstWindow, input); err != nil {
		t.Fatal(err)
	}
	nextWindow := domain.Quota{OrganizationID: "org-1", Metric: domain.MetricToken, Total: 100, WindowStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := repo.SetTarget(ctx, nextWindow, input); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("cross-window key reuse err=%v, want idempotency conflict", err)
	}
}

func TestConsumeIsIdempotentAndChecksBothPools(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	q := testQuota()
	if _, err := repo.SetTarget(ctx, q, setInput("member-1", "op-1", 10, 0)); err != nil {
		t.Fatal(err)
	}
	input := domain.ConsumeInput{OrganizationID: "org-1", MemberID: "member-1", Quantity: 10, IdempotencyKey: "use-1", SourceType: "ai", SourceID: "call-1"}
	if err := repo.Consume(ctx, q, input); err != nil {
		t.Fatal(err)
	}
	if err := repo.Consume(ctx, q, input); err != nil {
		t.Fatal(err)
	}
	input.IdempotencyKey, input.Quantity = "use-2", 1
	if err := repo.Consume(ctx, q, input); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("member err=%v", err)
	}
	var count int64
	if err := repo.db.Table("saas_usage_events").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("usage rows=%d", count)
	}
}

func TestConsumeIdempotencyKeyRejectsCrossWindowReplay(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	firstWindow := testQuota()
	if _, err := repo.SetTarget(ctx, firstWindow, setInput("member-1", "allocation-1", 10, 0)); err != nil {
		t.Fatal(err)
	}
	input := domain.ConsumeInput{OrganizationID: "org-1", MemberID: "member-1", Quantity: 1, IdempotencyKey: "use-1", SourceType: "ai", SourceID: "call-1"}
	if err := repo.Consume(ctx, firstWindow, input); err != nil {
		t.Fatal(err)
	}

	nextWindow := domain.Quota{OrganizationID: "org-1", Metric: domain.MetricToken, Total: 100, WindowStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
	if _, err := repo.SetTarget(ctx, nextWindow, setInput("member-1", "allocation-2", 10, 0)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Consume(ctx, nextWindow, input); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("cross-window key reuse err=%v, want idempotency conflict", err)
	}
	var count int64
	if err := repo.db.Table("saas_usage_events").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("usage rows=%d, want 1", count)
	}
}

func TestListRecentAuditUsesStableCursor(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	q := testQuota()
	for i, member := range []string{"member-1", "member-2", "member-3"} {
		if _, err := repo.SetTarget(ctx, q, setInput(member, "audit-"+string(rune('1'+i)), int64(10+i), 0)); err != nil {
			t.Fatal(err)
		}
	}
	first, err := repo.ListRecentAudit(ctx, "org-1", 2, "admin-1", "", nil)
	if err != nil || len(first.Items) != 2 || first.Next == nil {
		t.Fatalf("first page=%#v err=%v", first, err)
	}
	second, err := repo.ListRecentAudit(ctx, "org-1", 2, "admin-1", "", first.Next)
	if err != nil || len(second.Items) != 1 || second.Next != nil {
		t.Fatalf("second page=%#v err=%v", second, err)
	}
	seen := map[string]bool{}
	for _, item := range first.Items {
		seen[item.IdempotencyKey] = true
	}
	if seen[second.Items[0].IdempotencyKey] {
		t.Fatalf("cursor repeated audit event: first=%#v second=%#v", first, second)
	}
}
