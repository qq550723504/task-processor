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

func TestSubscriptionUsageRecordsWithoutLimit(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpsertEntitlement(context.Background(), "org-286", ModuleOSSStorage, EntitlementInput{
		Status: StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.CheckUsage(context.Background(), "org-286", ModuleOSSStorage, "storage_bytes", 2048)
	if err != nil {
		t.Fatalf("CheckUsage() error = %v", err)
	}
	if result.Limit != 0 || result.Used != 2048 || !result.Allowed {
		t.Fatalf("usage result = %#v, want unlimited recorded usage", result)
	}
	summary, err := svc.GetSummary(context.Background(), "org-286")
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == ModuleOSSStorage {
			if view.Used["storage_bytes"] != 2048 {
				t.Fatalf("summary storage_bytes = %d, want 2048", view.Used["storage_bytes"])
			}
			return
		}
	}
	t.Fatal("oss storage summary missing")
}

func TestSubscriptionUsageAuthorizeDoesNotRecordUntilUsageIsRecorded(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpsertEntitlement(context.Background(), "org-286", ModuleOSSStorage, EntitlementInput{
		Status: StatusActive,
		Limits: map[string]int{"storage_bytes": 4096},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.AuthorizeUsage(context.Background(), "org-286", ModuleOSSStorage, "storage_bytes", 2048)
	if err != nil {
		t.Fatalf("AuthorizeUsage() error = %v", err)
	}
	if !result.Allowed || result.Used != 2048 || result.Limit != 4096 {
		t.Fatalf("authorize result = %#v, want allowed 2048/4096", result)
	}
	summary, err := svc.GetSummary(context.Background(), "org-286")
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == ModuleOSSStorage && view.Used["storage_bytes"] != 0 {
			t.Fatalf("authorized usage should not be recorded, got %d", view.Used["storage_bytes"])
		}
	}
	if _, err := svc.RecordUsage(context.Background(), "org-286", ModuleOSSStorage, "storage_bytes", 2048); err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}
	summary, err = svc.GetSummary(context.Background(), "org-286")
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == ModuleOSSStorage {
			if view.Used["storage_bytes"] != 2048 {
				t.Fatalf("recorded storage_bytes = %d, want 2048", view.Used["storage_bytes"])
			}
			return
		}
	}
	t.Fatal("oss storage summary missing")
}

func TestSubscriptionUsageAuthorizeRejectsOverLimitWithoutRecording(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpsertEntitlement(context.Background(), "org-286", ModuleOSSStorage, EntitlementInput{
		Status: StatusActive,
		Limits: map[string]int{"storage_bytes": 1024},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.AuthorizeUsage(context.Background(), "org-286", ModuleOSSStorage, "storage_bytes", 2048)
	if !errors.Is(err, ErrSubscriptionQuotaExceed) {
		t.Fatalf("AuthorizeUsage() error = %v, want quota exceeded", err)
	}
	if result.Used != 2048 || result.Limit != 1024 {
		t.Fatalf("authorize result = %#v, want 2048/1024", result)
	}
	summary, err := svc.GetSummary(context.Background(), "org-286")
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range summary.Entitlements {
		if view.Module.Code == ModuleOSSStorage && view.Used["storage_bytes"] != 0 {
			t.Fatalf("rejected usage should not be recorded, got %d", view.Used["storage_bytes"])
		}
	}
}

func TestSubscriptionRecordUsageClampsNegativeAdjustmentsAtZero(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UpsertEntitlement(context.Background(), "org-286", ModuleOSSStorage, EntitlementInput{
		Status: StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecordUsage(context.Background(), "org-286", ModuleOSSStorage, "storage_bytes", 3); err != nil {
		t.Fatalf("seed usage: %v", err)
	}
	counter, err := svc.RecordUsage(context.Background(), "org-286", ModuleOSSStorage, "storage_bytes", -5)
	if err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}
	if counter.Used != 0 {
		t.Fatalf("used = %d, want 0", counter.Used)
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

func TestDefaultModulesIncludesOSSStorage(t *testing.T) {
	for _, module := range DefaultModules() {
		if module.Code == ModuleOSSStorage {
			if module.SortOrder != 60 || !module.Active {
				t.Fatalf("oss storage module = %#v", module)
			}
			return
		}
	}
	t.Fatal("default modules missing oss_storage")
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(NewMemRepository())
	if err != nil {
		t.Fatal(err)
	}
	return svc
}
