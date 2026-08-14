package listingsubscription

import (
	"context"
	"errors"
	"sort"
)

// UsageLedgerReconciliationCategory classifies a non-mutating ledger check.
type UsageLedgerReconciliationCategory string

const (
	usageLedgerUnknownContext = "unknown"

	UsageLedgerCommittedTotalMismatch UsageLedgerReconciliationCategory = "committed_total_mismatch"
	UsageLedgerReservedTotalMismatch  UsageLedgerReconciliationCategory = "reserved_total_mismatch"
	UsageLedgerOutboxDeliveryFailed   UsageLedgerReconciliationCategory = "outbox_delivery_failed"
	UsageLedgerOutboxMissing          UsageLedgerReconciliationCategory = "outbox_missing"
	UsageLedgerOutboxEventMissing     UsageLedgerReconciliationCategory = "outbox_event_missing"
)

// UsageLedgerReconciliationFinding contains only identifiers and a
// operator-safe reason. It never includes event metadata or outbox errors.
type UsageLedgerReconciliationFinding struct {
	Category   UsageLedgerReconciliationCategory
	TenantID   string
	ModuleCode string
	Metric     string
	PeriodKey  string
	EventID    string
	SafeReason string
}

// UsageLedgerReconciliationReport is an observational result. Reconciliation
// deliberately does not repair buckets, events, or outbox rows.
type UsageLedgerReconciliationReport struct {
	DryRun   bool
	Findings []UsageLedgerReconciliationFinding
}

// ReconcileUsageLedger reads the durable ledger and reports differences
// between bucket totals, lifecycle events, and outbox state. It is safe to run
// against production because it issues only SELECT queries.
func ReconcileUsageLedger(ctx context.Context, repo *GormRepository) (UsageLedgerReconciliationReport, error) {
	if repo == nil || repo.db == nil {
		return UsageLedgerReconciliationReport{}, errors.New("usage ledger repository is required")
	}

	var events []usageEventRow
	if err := repo.db.WithContext(ctx).Order("tenant_id ASC, module_code ASC, period_key ASC, metric ASC, event_id ASC").Find(&events).Error; err != nil {
		return UsageLedgerReconciliationReport{}, err
	}
	var buckets []usageBucketRow
	if err := repo.db.WithContext(ctx).Order("tenant_id ASC, module_code ASC, period_key ASC, metric ASC").Find(&buckets).Error; err != nil {
		return UsageLedgerReconciliationReport{}, err
	}
	var outbox []usageEventOutboxRow
	if err := repo.db.WithContext(ctx).Order("event_id ASC, id ASC").Find(&outbox).Error; err != nil {
		return UsageLedgerReconciliationReport{}, err
	}

	return reconcileUsageLedgerRows(events, buckets, outbox), nil
}

type usageLedgerBucketKey struct {
	tenantID   string
	moduleCode string
	periodKey  string
	metric     string
}

type usageLedgerTotals struct {
	committed        int64
	reserved         int64
	committedEventID string
	reservedEventID  string
}

func reconcileUsageLedgerRows(events []usageEventRow, buckets []usageBucketRow, outbox []usageEventOutboxRow) UsageLedgerReconciliationReport {
	report := UsageLedgerReconciliationReport{DryRun: true, Findings: []UsageLedgerReconciliationFinding{}}
	totalsByBucket := make(map[usageLedgerBucketKey]*usageLedgerTotals, len(events))
	eventByID := make(map[string]usageEventRow, len(events))
	outboxByEventID := make(map[string]usageEventOutboxRow, len(outbox))

	for _, event := range events {
		key := usageLedgerKey(event.TenantID, event.ModuleCode, event.PeriodKey, event.Metric)
		totals := totalsByBucket[key]
		if totals == nil {
			totals = &usageLedgerTotals{}
			totalsByBucket[key] = totals
		}
		eventByID[event.EventID] = event
		switch UsageEventStatus(event.Status) {
		case UsageEventCommitted, UsageEventReversed:
			totals.committed += event.Quantity
			if totals.committedEventID == "" || event.EventID < totals.committedEventID {
				totals.committedEventID = event.EventID
			}
		case UsageEventReserved:
			totals.reserved += event.Quantity
			if totals.reservedEventID == "" || event.EventID < totals.reservedEventID {
				totals.reservedEventID = event.EventID
			}
		}
	}
	for _, item := range outbox {
		outboxByEventID[item.EventID] = item
	}

	for _, bucket := range buckets {
		key := usageLedgerKey(bucket.TenantID, bucket.ModuleCode, bucket.PeriodKey, bucket.Metric)
		totals := totalsByBucket[key]
		if totals == nil {
			totals = &usageLedgerTotals{}
		}
		if bucket.Committed != totals.committed {
			report.Findings = append(report.Findings, usageLedgerBucketFinding(UsageLedgerCommittedTotalMismatch, bucket, totals.committedEventID, "bucket committed total differs from committed and reversed event sum"))
		}
		if bucket.Reserved != totals.reserved {
			report.Findings = append(report.Findings, usageLedgerBucketFinding(UsageLedgerReservedTotalMismatch, bucket, totals.reservedEventID, "bucket reserved total differs from reserved event sum"))
		}
		delete(totalsByBucket, key)
	}
	for key, totals := range totalsByBucket {
		if totals.committed == 0 && totals.reserved == 0 {
			continue
		}
		if totals.committed != 0 {
			report.Findings = append(report.Findings, UsageLedgerReconciliationFinding{Category: UsageLedgerCommittedTotalMismatch, TenantID: key.tenantID, ModuleCode: key.moduleCode, Metric: key.metric, PeriodKey: key.periodKey, EventID: totals.committedEventID, SafeReason: "events have no usage bucket"})
		}
		if totals.reserved != 0 {
			report.Findings = append(report.Findings, UsageLedgerReconciliationFinding{Category: UsageLedgerReservedTotalMismatch, TenantID: key.tenantID, ModuleCode: key.moduleCode, Metric: key.metric, PeriodKey: key.periodKey, EventID: totals.reservedEventID, SafeReason: "events have no usage bucket"})
		}
	}

	for _, event := range events {
		item, ok := outboxByEventID[event.EventID]
		if !ok {
			report.Findings = append(report.Findings, usageLedgerEventFinding(UsageLedgerOutboxMissing, event, "ledger event has no outbox item"))
			continue
		}
		if item.Status == "failed" {
			report.Findings = append(report.Findings, usageLedgerEventFinding(UsageLedgerOutboxDeliveryFailed, event, "outbox delivery requires retry; error detail is intentionally omitted"))
		}
	}
	for _, item := range outbox {
		if _, ok := eventByID[item.EventID]; !ok {
			report.Findings = append(report.Findings, UsageLedgerReconciliationFinding{Category: UsageLedgerOutboxEventMissing, TenantID: usageLedgerUnknownContext, ModuleCode: usageLedgerUnknownContext, Metric: usageLedgerUnknownContext, PeriodKey: usageLedgerUnknownContext, EventID: item.EventID, SafeReason: "outbox item references no ledger event; ledger context is unavailable"})
		}
	}

	sort.Slice(report.Findings, func(i, j int) bool {
		left, right := report.Findings[i], report.Findings[j]
		if left.TenantID != right.TenantID {
			return left.TenantID < right.TenantID
		}
		if left.ModuleCode != right.ModuleCode {
			return left.ModuleCode < right.ModuleCode
		}
		if left.PeriodKey != right.PeriodKey {
			return left.PeriodKey < right.PeriodKey
		}
		if left.Metric != right.Metric {
			return left.Metric < right.Metric
		}
		if left.EventID != right.EventID {
			return left.EventID < right.EventID
		}
		return left.Category < right.Category
	})
	return report
}

func usageLedgerKey(tenantID, moduleCode, periodKey, metric string) usageLedgerBucketKey {
	return usageLedgerBucketKey{tenantID: tenantID, moduleCode: moduleCode, periodKey: periodKey, metric: metric}
}

func usageLedgerBucketFinding(category UsageLedgerReconciliationCategory, bucket usageBucketRow, eventID, safeReason string) UsageLedgerReconciliationFinding {
	return UsageLedgerReconciliationFinding{Category: category, TenantID: bucket.TenantID, ModuleCode: bucket.ModuleCode, Metric: bucket.Metric, PeriodKey: bucket.PeriodKey, EventID: eventID, SafeReason: safeReason}
}

func usageLedgerEventFinding(category UsageLedgerReconciliationCategory, event usageEventRow, safeReason string) UsageLedgerReconciliationFinding {
	return UsageLedgerReconciliationFinding{Category: category, TenantID: event.TenantID, ModuleCode: event.ModuleCode, Metric: event.Metric, PeriodKey: event.PeriodKey, EventID: event.EventID, SafeReason: safeReason}
}
