package listingsubscription

import (
	"context"
	"strings"
	"testing"
)

func TestUsageLedgerReconciliationDoesNotReportValidLifecycleOrIdempotentReplay(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 20})
	ledger := NewGormUsageLedger(repo)

	committedReservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-committed", 1))
	if err != nil {
		t.Fatalf("Reserve(committed) error = %v", err)
	}
	if _, err := ledger.Commit(ctx, committedReservation.Event.EventID); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	replay, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-committed", 1))
	if err != nil {
		t.Fatalf("Reserve(replay) error = %v", err)
	}
	if !replay.Existing || replay.Event.EventID != committedReservation.Event.EventID {
		t.Fatalf("Reserve(replay) = %#v, want existing committed event %q", replay, committedReservation.Event.EventID)
	}

	if _, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-reserved", 1)); err != nil {
		t.Fatalf("Reserve(reserved) error = %v", err)
	}
	releasedReservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-released", 1))
	if err != nil {
		t.Fatalf("Reserve(released) error = %v", err)
	}
	if _, err := ledger.Release(ctx, releasedReservation.Event.EventID, "customer cancelled"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	report, err := ReconcileUsageLedger(ctx, repo)
	if err != nil {
		t.Fatalf("ReconcileUsageLedger() error = %v", err)
	}
	if !report.DryRun {
		t.Fatal("report DryRun = false, want true")
	}
	if len(report.Findings) != 0 {
		t.Fatalf("report findings = %#v, want no false positives for valid committed, reserved, released, and replayed events", report.Findings)
	}

	var eventCount, bucketCount, outboxCount int64
	if err := db.Model(&usageEventRow{}).Count(&eventCount).Error; err != nil {
		t.Fatalf("count events after reconciliation: %v", err)
	}
	if err := db.Model(&usageBucketRow{}).Count(&bucketCount).Error; err != nil {
		t.Fatalf("count buckets after reconciliation: %v", err)
	}
	if err := db.Model(&usageEventOutboxRow{}).Count(&outboxCount).Error; err != nil {
		t.Fatalf("count outbox after reconciliation: %v", err)
	}
	if eventCount != 3 || bucketCount != 1 || outboxCount != 3 {
		t.Fatalf("reconciliation changed ledger rows: events=%d buckets=%d outbox=%d, want 3/1/3", eventCount, bucketCount, outboxCount)
	}
}

func TestUsageLedgerReconciliationReportsBucketAndOutboxMismatchesWithSafeContext(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 20})
	ledger := NewGormUsageLedger(repo)

	committedReservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-committed", 1))
	if err != nil {
		t.Fatalf("Reserve(committed) error = %v", err)
	}
	if _, err := ledger.Commit(ctx, committedReservation.Event.EventID); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	reservedReservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-reserved", 1))
	if err != nil {
		t.Fatalf("Reserve(reserved) error = %v", err)
	}
	if err := db.Model(&usageBucketRow{}).
		Where("tenant_id = ? AND module_code = ? AND period_key = ? AND metric = ?", "tenant-17", "studio", "2026-08", "studio_design_jobs_succeeded").
		Updates(map[string]any{"committed": 9, "reserved": 8}).Error; err != nil {
		t.Fatalf("tamper bucket: %v", err)
	}
	if err := db.Model(&usageEventOutboxRow{}).Where("event_id = ?", committedReservation.Event.EventID).Update("status", "failed").Error; err != nil {
		t.Fatalf("mark outbox failed: %v", err)
	}

	report, err := ReconcileUsageLedger(ctx, repo)
	if err != nil {
		t.Fatalf("ReconcileUsageLedger() error = %v", err)
	}
	if !report.DryRun {
		t.Fatal("report DryRun = false, want true")
	}
	want := map[UsageLedgerReconciliationCategory]string{
		UsageLedgerCommittedTotalMismatch: committedReservation.Event.EventID,
		UsageLedgerReservedTotalMismatch:  reservedReservation.Event.EventID,
		UsageLedgerOutboxDeliveryFailed:   committedReservation.Event.EventID,
	}
	if len(report.Findings) != len(want) {
		t.Fatalf("report findings = %#v, want one finding for each mismatch category", report.Findings)
	}
	for _, finding := range report.Findings {
		wantEventID, ok := want[finding.Category]
		if !ok {
			t.Fatalf("unexpected reconciliation finding = %#v", finding)
		}
		if finding.TenantID != "tenant-17" || finding.Metric != "studio_design_jobs_succeeded" || finding.EventID != wantEventID || finding.SafeReason == "" {
			t.Fatalf("finding = %#v, want safe tenant/metric/event context", finding)
		}
		delete(want, finding.Category)
	}
	if len(want) != 0 {
		t.Fatalf("missing reconciliation categories = %#v", want)
	}
}

func TestUsageLedgerReconciliationReportsMissingAndOrphanOutboxWithSafeContext(t *testing.T) {
	ctx := context.Background()
	db := openUsageLedgerTestDB(t)
	repo := NewGormRepository(db)
	seedUsageLedgerEntitlement(t, repo, "tenant-17", "studio", map[string]int{"studio_design_jobs_succeeded": 20})
	ledger := NewGormUsageLedger(repo)

	reservation, err := ledger.Reserve(ctx, usageLedgerReserveInput("tenant-17", "request-missing-outbox", 1))
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := db.Where("event_id = ?", reservation.Event.EventID).Delete(&usageEventOutboxRow{}).Error; err != nil {
		t.Fatalf("delete outbox: %v", err)
	}
	if err := db.Create(&usageEventOutboxRow{EventID: "event-orphan", Status: "failed", LastError: "customer@example.com provider token: secret"}).Error; err != nil {
		t.Fatalf("create orphan outbox: %v", err)
	}

	report, err := ReconcileUsageLedger(ctx, repo)
	if err != nil {
		t.Fatalf("ReconcileUsageLedger() error = %v", err)
	}
	want := map[UsageLedgerReconciliationCategory]UsageLedgerReconciliationFinding{
		UsageLedgerOutboxMissing: {
			Category: UsageLedgerOutboxMissing, TenantID: "tenant-17", Metric: "studio_design_jobs_succeeded", EventID: reservation.Event.EventID,
		},
		UsageLedgerOutboxEventMissing: {
			Category: UsageLedgerOutboxEventMissing, TenantID: "unknown", Metric: "unknown", EventID: "event-orphan",
		},
	}
	if len(report.Findings) != len(want) {
		t.Fatalf("report findings = %#v, want missing and orphan outbox findings", report.Findings)
	}
	for _, finding := range report.Findings {
		expected, ok := want[finding.Category]
		if !ok {
			t.Fatalf("unexpected reconciliation finding = %#v", finding)
		}
		if finding.TenantID != expected.TenantID || finding.Metric != expected.Metric || finding.EventID != expected.EventID || finding.SafeReason == "" {
			t.Fatalf("finding = %#v, want safe tenant/metric/event context", finding)
		}
		if strings.Contains(finding.SafeReason, "customer@example.com") || strings.Contains(finding.SafeReason, "secret") || strings.Contains(finding.SafeReason, "metadata") {
			t.Fatalf("finding SafeReason = %q, must not expose outbox error or event metadata", finding.SafeReason)
		}
		delete(want, finding.Category)
	}
	if len(want) != 0 {
		t.Fatalf("missing reconciliation findings = %#v", want)
	}
}

func TestUsageLedgerReconciliationReportsLifecycleMismatch(t *testing.T) {
	event := usageEventRow{EventID: "event-mismatch", TenantID: "tenant-17", ModuleCode: ModuleStudio, Metric: usageMetricStudioDesignJobsSucceeded, Quantity: 1, PeriodKey: "2026-08", Status: string(UsageEventReserved)}
	report := reconcileUsageLedgerRows([]usageEventRow{event}, []usageBucketRow{{TenantID: "tenant-17", ModuleCode: ModuleStudio, PeriodKey: "2026-08", Metric: usageMetricStudioDesignJobsSucceeded, Reserved: 1}}, []usageEventOutboxRow{{EventID: event.EventID, Status: "sent"}})
	if len(report.Findings) != 1 || report.Findings[0].Category != UsageLedgerOutboxLifecycleMismatch {
		t.Fatalf("findings = %#v, want lifecycle mismatch only", report.Findings)
	}
}
