package listingsubscription

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIInvocationUsageAdapter is the narrow error-only adapter consumed by the
// aicapability recorder; the repository method remains useful to callers that
// need the committed event for tracing or reconciliation.
type AIInvocationUsageAdapter struct{ Repository *GormRepository }

func (a AIInvocationUsageAdapter) SettleAIInvocationUsage(ctx context.Context, tenantID, memberID, invocationID string, totalTokens int64, occurredAt time.Time) error {
	if a.Repository == nil {
		return ErrUsageLedgerNotConfigured
	}
	_, err := a.Repository.SettleAIInvocationUsage(ctx, tenantID, memberID, invocationID, totalTokens, occurredAt)
	return err
}

// UsagePeriodKeyForWindow is the stable commercial bucket identity for an
// entitlement window. It is deliberately derived from the window, not from a
// wall-clock month.
func UsagePeriodKeyForWindow(start, end time.Time) string {
	return fmt.Sprintf("ai:%d:%d", start.UTC().UnixNano(), end.UTC().UnixNano())
}

type memberUsageRow struct {
	Quantity int64 `gorm:"column:quantity"`
}

type memberAllocationRow struct {
	OrganizationID string    `gorm:"column:organization_id"`
	MemberID       string    `gorm:"column:member_id"`
	Metric         string    `gorm:"column:metric"`
	Allocated      int64     `gorm:"column:allocated"`
	Active         bool      `gorm:"column:active"`
	WindowStart    time.Time `gorm:"column:window_start"`
	WindowEnd      time.Time `gorm:"column:window_end"`
}

func (memberAllocationRow) TableName() string { return "account_member_token_allocations" }

type organizationAllocationLockRow struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (organizationAllocationLockRow) TableName() string { return "account_member_token_locks" }

func (memberUsageRow) TableName() string { return "saas_usage_events" }

// SumCommittedMemberUsage reads consumption projection from the commercial
// event ledger. It does not maintain a second usage fact table.
func (r *GormRepository) SumCommittedMemberUsage(ctx context.Context, tx *gorm.DB, tenantID, memberID, metric string, start, end time.Time) (int64, error) {
	if r == nil || tx == nil || tenantID == "" || memberID == "" || metric == "" || start.IsZero() || !end.After(start) {
		return 0, ErrUsageInvalidInput
	}
	var total int64
	query := tx.WithContext(ctx).Model(&memberUsageRow{}).Where("tenant_id = ? AND member_id = ? AND metric = ? AND status = ? AND occurred_at >= ? AND occurred_at < ?", tenantID, memberID, metric, string(UsageEventCommitted), start.UTC(), end.UTC())
	if err := query.Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *GormRepository) SumCommittedUsage(ctx context.Context, tx *gorm.DB, tenantID, metric string, start, end time.Time) (int64, error) {
	if r == nil || tx == nil || tenantID == "" || metric == "" || start.IsZero() || !end.After(start) {
		return 0, ErrUsageInvalidInput
	}
	var total int64
	query := tx.WithContext(ctx).Model(&memberUsageRow{}).Where("tenant_id = ? AND metric = ? AND status = ? AND occurred_at >= ? AND occurred_at < ?", tenantID, metric, string(UsageEventCommitted), start.UTC(), end.UTC())
	if err := query.Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// SettleAIInvocationUsage records one successful, observed AI invocation in
// the commercial ledger. The invocation identity is checked by source_type /
// source_id as well as by the period-bound idempotency key, so a retry cannot
// move an invocation to another entitlement window or change its quantity.
func (r *GormRepository) SettleAIInvocationUsage(ctx context.Context, tenantID, memberID, invocationID string, totalTokens int64, occurredAt time.Time) (UsageEvent, error) {
	tenantID = strings.TrimSpace(tenantID)
	memberID = strings.TrimSpace(memberID)
	invocationID = strings.TrimSpace(invocationID)
	if r == nil || r.db == nil || tenantID == "" || memberID == "" || invocationID == "" || totalTokens <= 0 || occurredAt.IsZero() {
		return UsageEvent{}, ErrUsageInvalidInput
	}
	var result UsageEvent
	err := runUsageLedgerTransaction(ctx, r.db, func(tx *gorm.DB) error {
		entitlement, err := loadEffectiveUsageEntitlement(tx, tenantID, ModuleListingKit)
		if err != nil {
			return err
		}
		periodKey, err := entitlementWindowPeriodKey(entitlement)
		if err != nil {
			return err
		}
		var existing usageEventRow
		lookup := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND metric = ? AND source_type = ? AND source_id = ?", tenantID, usageMetricAITokens, "ai_invocation", invocationID).Take(&existing)
		if lookup.Error == nil {
			if existing.Quantity != totalTokens || existing.MemberID != memberID || existing.PeriodKey != periodKey {
				return &UsageDuplicateIdentityError{TenantID: tenantID, IdempotencyKey: existing.IdempotencyKey}
			}
			if existing.Status == string(UsageEventCommitted) {
				result = usageEventFromRow(existing)
				return nil
			}
			if existing.Status != string(UsageEventReserved) {
				return &UsageDuplicateIdentityError{TenantID: tenantID, IdempotencyKey: existing.IdempotencyKey}
			}
			ledger := &gormUsageLedger{repo: &GormRepository{db: tx}}
			committed, commitErr := ledger.Commit(ctx, existing.EventID)
			result = committed
			return commitErr
		}
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return lookup.Error
		}
		// Serialize member allocation changes and member-scoped usage on the
		// commercial owner. Membership facts remain in the membership owner;
		// this row is only the commercial reservation entitlement.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&organizationAllocationLockRow{OrganizationID: tenantID, UpdatedAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		var lock organizationAllocationLockRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ?", tenantID).Take(&lock).Error; err != nil {
			return err
		}
		var allocation memberAllocationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ? AND metric = ?", tenantID, memberID, "token").Take(&allocation).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUsageQuotaExceeded
		} else if err != nil {
			return err
		}
		if !allocation.Active || !allocation.WindowStart.UTC().Equal(entitlement.StartsAt.UTC()) || !allocation.WindowEnd.UTC().Equal(entitlement.ExpiresAt.UTC()) {
			return ErrUsageQuotaExceeded
		}
		memberConsumed, err := r.SumCommittedMemberUsage(ctx, tx, tenantID, memberID, usageMetricAITokens, *entitlement.StartsAt, *entitlement.ExpiresAt)
		if err != nil {
			return err
		}
		if totalTokens > allocation.Allocated-memberConsumed {
			return ErrUsageQuotaExceeded
		}
		input := ReserveUsageInput{
			TenantID: tenantID, ModuleCode: ModuleListingKit, Metric: usageMetricAITokens,
			Quantity: totalTokens, PeriodKey: periodKey, SourceType: "ai_invocation", SourceID: invocationID,
			MemberID: memberID, IdempotencyKey: fmt.Sprintf("ai_invocation:%s:%s", periodKey, invocationID), OccurredAt: occurredAt.UTC(),
		}
		ledger := &gormUsageLedger{repo: &GormRepository{db: tx}}
		reserved, err := ledger.Reserve(ctx, input)
		if err != nil {
			return err
		}
		result, err = ledger.Commit(ctx, reserved.Event.EventID)
		return err
	})
	return result, err
}

func entitlementWindowPeriodKey(entitlement tenantEntitlementRow) (string, error) {
	if entitlement.StartsAt == nil || entitlement.ExpiresAt == nil || !entitlement.ExpiresAt.After(*entitlement.StartsAt) {
		return "", &UsageValidationError{Field: "entitlement_window"}
	}
	return UsagePeriodKeyForWindow(*entitlement.StartsAt, *entitlement.ExpiresAt), nil
}
