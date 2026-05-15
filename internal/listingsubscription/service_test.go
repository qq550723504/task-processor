package listingsubscription

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSubscriptionGuardRequiresConfiguredEntitlement(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.Check(context.Background(), "org-286", ModuleStudio)
	if !errors.Is(err, ErrSubscriptionRequired) {
		t.Fatalf("Check() error = %v, want subscription required", err)
	}
	if result.Reason != "not_configured" {
		t.Fatalf("reason = %q, want not_configured", result.Reason)
	}
}

func TestSubscriptionGuardAllowsActiveEntitlement(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpsertEntitlement(context.Background(), "org-286", ModuleStudio, EntitlementInput{Status: StatusActive})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Check(context.Background(), "org-286", ModuleStudio)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.Allowed {
		t.Fatal("expected active entitlement to be allowed")
	}
}

func TestSubscriptionGuardRejectsExpiredEntitlement(t *testing.T) {
	svc := newTestService(t)
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	expiredAt := now.Add(-time.Hour)
	_, err := svc.UpsertEntitlement(context.Background(), "org-286", ModuleStudio, EntitlementInput{
		Status: StatusActive, ExpiresAt: &expiredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.Check(context.Background(), "org-286", ModuleStudio)
	if !errors.Is(err, ErrSubscriptionRequired) {
		t.Fatalf("Check() error = %v, want subscription required", err)
	}
	if result.Reason != StatusExpired {
		t.Fatalf("reason = %q, want expired", result.Reason)
	}
}

func TestSubscriptionGuardRejectsQuotaExceeded(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpsertEntitlement(context.Background(), "org-286", ModuleStudio, EntitlementInput{
		Status: StatusActive,
		Limits: map[string]int{"design_jobs": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CheckUsage(context.Background(), "org-286", ModuleStudio, "design_jobs", 1); err != nil {
		t.Fatalf("first usage error = %v", err)
	}
	result, err := svc.CheckUsage(context.Background(), "org-286", ModuleStudio, "design_jobs", 1)
	if !errors.Is(err, ErrSubscriptionQuotaExceed) {
		t.Fatalf("second usage error = %v, want quota exceeded", err)
	}
	if result.Limit != 1 || result.Used != 2 {
		t.Fatalf("quota result = limit %d used %d, want 1/2", result.Limit, result.Used)
	}
}

func TestSubscriptionUsageAdjustmentWritesAuditLog(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpsertEntitlementWithAudit(context.Background(), "org-286", ModuleStudio, EntitlementInput{
		Status: StatusActive,
		Limits: map[string]int{"design_jobs": 10},
	}, "admin-1", "manual open")
	if err != nil {
		t.Fatal(err)
	}
	counter, err := svc.SetUsage(context.Background(), "org-286", ModuleStudio, UsageAdjustmentInput{
		PeriodKey: "2026-05",
		Metric:    "design_jobs",
		Used:      3,
		Reason:    "manual correction",
	}, "admin-1")
	if err != nil {
		t.Fatal(err)
	}
	if counter.Used != 3 {
		t.Fatalf("used = %d, want 3", counter.Used)
	}
	logs, err := svc.ListAuditLogs(context.Background(), "org-286", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 {
		t.Fatalf("audit logs = %d, want 2", len(logs))
	}
	if logs[0].Action != "usage_set" || logs[0].ActorID != "admin-1" || logs[0].Reason != "manual correction" {
		t.Fatalf("latest audit log = %#v", logs[0])
	}
}

func TestSubscriptionSummaryIncludesUnconfiguredModules(t *testing.T) {
	svc := newTestService(t)
	summary, err := svc.GetSummary(context.Background(), "org-286")
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.Entitlements) != len(DefaultModules()) {
		t.Fatalf("summary entitlements = %d, want %d", len(summary.Entitlements), len(DefaultModules()))
	}
	if summary.Entitlements[0].Reason != "not_configured" {
		t.Fatalf("first reason = %q, want not_configured", summary.Entitlements[0].Reason)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(NewMemRepository())
	if err != nil {
		t.Fatal(err)
	}
	return svc
}
