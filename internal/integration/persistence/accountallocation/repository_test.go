package accountallocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	domain "task-processor/internal/accountallocation"
)

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
	if page.Enterprise.Allocated != 0 || page.Enterprise.Unallocated != 80 || page.Enterprise.Consumed != 20 {
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
