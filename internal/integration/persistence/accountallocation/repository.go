package accountallocation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	domain "task-processor/internal/accountallocation"
	"task-processor/internal/listingsubscription"
)

const (
	allocationTable = "account_member_token_allocations"
	operationTable  = "account_member_token_operations"
	lockTable       = "account_member_token_locks"
	auditTable      = "account_member_token_audit_events"
)

type allocationRow struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey;size:128"`
	MemberID       string    `gorm:"column:member_id;primaryKey;size:128"`
	Metric         string    `gorm:"column:metric;primaryKey;size:32"`
	Allocated      int64     `gorm:"column:allocated;not null"`
	Version        int64     `gorm:"column:version;not null"`
	Active         bool      `gorm:"column:active;not null"`
	WindowStart    time.Time `gorm:"column:window_start"`
	WindowEnd      time.Time `gorm:"column:window_end"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (allocationRow) TableName() string { return allocationTable }

type operationRow struct {
	IdempotencyKey string    `gorm:"column:idempotency_key;primaryKey;size:128"`
	OrganizationID string    `gorm:"column:organization_id;not null"`
	MemberID       string    `gorm:"column:member_id;not null"`
	Fingerprint    string    `gorm:"column:fingerprint;not null;size:64"`
	Target         int64     `gorm:"column:target;not null"`
	Version        int64     `gorm:"column:version;not null"`
	Allocated      int64     `gorm:"column:allocated;not null"`
	Consumed       int64     `gorm:"column:consumed;not null"`
	Active         bool      `gorm:"column:active;not null"`
	WindowStart    time.Time `gorm:"column:window_start;not null"`
	WindowEnd      time.Time `gorm:"column:window_end;not null"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (operationRow) TableName() string { return operationTable }

type lockRow struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey;size:128"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (lockRow) TableName() string { return lockTable }

type auditRow struct {
	ID             uint      `gorm:"column:id;primaryKey;autoIncrement"`
	OrganizationID string    `gorm:"column:organization_id;not null;index"`
	ActorID        string    `gorm:"column:actor_id;not null"`
	MemberID       string    `gorm:"column:member_id;not null"`
	Operation      string    `gorm:"column:operation;not null;size:32"`
	Target         int64     `gorm:"column:target;not null"`
	Allocated      int64     `gorm:"column:allocated;not null"`
	Consumed       int64     `gorm:"column:consumed;not null"`
	Version        int64     `gorm:"column:version;not null"`
	IdempotencyKey string    `gorm:"column:idempotency_key;not null;uniqueIndex"`
	CreatedAt      time.Time `gorm:"column:created_at;index"`
}

func (auditRow) TableName() string { return auditTable }

type commercialUsageEventRow struct {
	IdempotencyKey string    `gorm:"column:idempotency_key"`
	TenantID       string    `gorm:"column:tenant_id"`
	MemberID       string    `gorm:"column:member_id"`
	Metric         string    `gorm:"column:metric"`
	Quantity       int64     `gorm:"column:quantity"`
	SourceType     string    `gorm:"column:source_type"`
	SourceID       string    `gorm:"column:source_id"`
	PeriodKey      string    `gorm:"column:period_key"`
	Status         string    `gorm:"column:status"`
	OccurredAt     time.Time `gorm:"column:occurred_at"`
}

func (commercialUsageEventRow) TableName() string { return "saas_usage_events" }

type Repository struct {
	db         *gorm.DB
	commercial *listingsubscription.GormRepository
}

func (r *Repository) ListRecentAudit(ctx context.Context, organizationID string, limit int, actor, operation string, after *domain.AuditPosition) (domain.AuditPage, error) {
	if r == nil || r.db == nil || organizationID == "" || limit < 1 {
		return domain.AuditPage{}, domain.ErrInvalidRequest
	}
	query := r.db.WithContext(ctx).Where("organization_id = ?", organizationID)
	if actor != "" {
		query = query.Where("actor_id = ?", actor)
	}
	if operation != "" {
		query = query.Where("operation = ?", operation)
	}
	if after != nil {
		if !after.Valid() {
			return domain.AuditPage{}, domain.ErrInvalidRequest
		}
		query = query.Where("(created_at < ?) OR (created_at = ? AND idempotency_key < ?)", after.CreatedAt, after.CreatedAt, after.IdempotencyKey)
	}
	var rows []auditRow
	// Read one extra row so the projection can distinguish an exhausted stream
	// from a page that needs a cursor.
	if err := query.Order("created_at DESC, idempotency_key DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return domain.AuditPage{}, mapError(err)
	}
	page := domain.AuditPage{Items: make([]domain.AuditEvent, 0, limit)}
	for i, row := range rows {
		if i == limit {
			position := page.Items[len(page.Items)-1].Position()
			page.Next = &position
			break
		}
		page.Items = append(page.Items, domain.AuditEvent{OrganizationID: row.OrganizationID, ActorID: row.ActorID, MemberID: row.MemberID, Operation: row.Operation, Target: row.Target, Allocated: row.Allocated, Consumed: row.Consumed, Version: row.Version, IdempotencyKey: row.IdempotencyKey, CreatedAt: row.CreatedAt.UTC().Truncate(time.Microsecond)})
	}
	return page, nil
}

func New(db *gorm.DB) (*Repository, error) {
	if db == nil {
		return nil, domain.ErrUnavailable
	}
	return &Repository{db: db, commercial: listingsubscription.NewGormRepository(db)}, nil
}

func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return domain.ErrUnavailable
	}
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		return err
	}
	return db.AutoMigrate(&allocationRow{}, &operationRow{}, &lockRow{}, &auditRow{})
}

func (r *Repository) Snapshot(ctx context.Context, quota domain.Quota) (domain.Snapshot, error) {
	if r == nil || r.db == nil || !validQuota(quota) {
		return domain.Snapshot{}, domain.ErrInvalidRequest
	}
	var rows []allocationRow
	if err := r.db.WithContext(ctx).Where("organization_id = ? AND metric = ? AND window_start = ? AND window_end = ?", quota.OrganizationID, domain.MetricToken, quota.WindowStart, quota.WindowEnd).Order("member_id").Find(&rows).Error; err != nil {
		return domain.Snapshot{}, mapError(err)
	}
	consumed, err := r.sumUsage(ctx, r.db, quota)
	if err != nil {
		return domain.Snapshot{}, err
	}
	var allocated int64
	var consumedByInactive int64
	result := domain.Snapshot{OrganizationID: quota.OrganizationID, Metric: domain.MetricToken, WindowStart: quota.WindowStart, WindowEnd: quota.WindowEnd, Allocations: make([]domain.Allocation, 0, len(rows))}
	for _, row := range rows {
		memberConsumed, err := r.memberConsumed(ctx, quota, row.MemberID)
		if err != nil {
			return domain.Snapshot{}, err
		}
		value := allocationFromRow(row, memberConsumed)
		if row.Active {
			allocated, err = checkedAdd(allocated, row.Allocated)
			if err != nil {
				return domain.Snapshot{}, err
			}
		} else {
			// A removed member's unused reservation is released, but its
			// already-consumed floor remains unavailable to replacement members.
			consumedByInactive, err = checkedAdd(consumedByInactive, memberConsumed)
			if err != nil {
				return domain.Snapshot{}, err
			}
		}
		result.Allocations = append(result.Allocations, value)
	}
	if allocated > quota.Total || consumed > quota.Total {
		return domain.Snapshot{}, domain.ErrQuotaExceeded
	}
	unallocated := quota.Total - allocated - consumedByInactive
	if unallocated < 0 {
		return domain.Snapshot{}, domain.ErrQuotaExceeded
	}
	result.Enterprise = domain.EnterpriseView{Total: quota.Total, Allocated: allocated, Unallocated: unallocated, Consumed: consumed}
	return result, nil
}

func (r *Repository) SetTarget(ctx context.Context, quota domain.Quota, input domain.SetTargetInput) (domain.Allocation, error) {
	if r == nil || r.db == nil || !validQuota(quota) || input.OrganizationID != quota.OrganizationID || input.MemberID == "" || input.Target < 0 || input.ExpectedVersion < 0 {
		return domain.Allocation{}, domain.ErrInvalidRequest
	}
	fingerprint := setFingerprint(input)
	var result domain.Allocation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockOrganization(tx, quota.OrganizationID); err != nil {
			return err
		}
		var operation operationRow
		err := tx.Where("idempotency_key = ?", input.IdempotencyKey).Take(&operation).Error
		if err == nil {
			if operation.Fingerprint != fingerprint || operation.OrganizationID != input.OrganizationID || operation.MemberID != input.MemberID || !operation.WindowStart.UTC().Equal(quota.WindowStart.UTC()) || !operation.WindowEnd.UTC().Equal(quota.WindowEnd.UTC()) {
				return domain.ErrIdempotencyConflict
			}
			result = allocationFromOperation(operation)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return mapError(err)
		}
		var row allocationRow
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ? AND metric = ? AND window_start = ? AND window_end = ?", quota.OrganizationID, input.MemberID, domain.MetricToken, quota.WindowStart, quota.WindowEnd).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			row = allocationRow{OrganizationID: quota.OrganizationID, MemberID: input.MemberID, Metric: domain.MetricToken, WindowStart: quota.WindowStart, WindowEnd: quota.WindowEnd, Version: 0}
		} else if err != nil {
			return mapError(err)
		}
		if row.Version != input.ExpectedVersion {
			return domain.ErrConflict
		}
		consumed, err := sumMemberConsumed(tx, quota, input.MemberID)
		if err != nil {
			return err
		}
		if input.Target > 0 && input.Target < consumed {
			return domain.ErrConsumedFloor
		}
		var allocated int64
		if err := tx.Model(&allocationRow{}).Where("organization_id = ? AND metric = ? AND window_start = ? AND window_end = ? AND active = ? AND member_id <> ?", quota.OrganizationID, domain.MetricToken, quota.WindowStart, quota.WindowEnd, true, input.MemberID).Select("COALESCE(SUM(allocated), 0)").Scan(&allocated).Error; err != nil {
			return mapError(err)
		}
		// The active rows represent unconsumed reservations. Consumed quota is
		// owned by the commercial ledger and must still count even when a
		// removed member's allocation has been made inactive. Otherwise a
		// replacement member could reuse tokens already consumed by the removed
		// member.
		enterpriseConsumed, err := sumUsage(tx, quota)
		if err != nil {
			return err
		}
		if input.Target > quota.Total-enterpriseConsumed-allocated {
			return domain.ErrQuotaExceeded
		}
		now := time.Now().UTC().Truncate(time.Microsecond)
		storedTarget := input.Target
		active := input.Target > 0
		// target=0 is the explicit revoke operation. Preserve already consumed
		// usage in the allocation fact while releasing only the unconsumed
		// reservation and preventing any future member consumption.
		if !active && consumed > 0 {
			storedTarget = consumed
		}
		row.Allocated, row.Version, row.Active, row.WindowStart, row.WindowEnd, row.UpdatedAt = storedTarget, input.ExpectedVersion+1, active, quota.WindowStart, quota.WindowEnd, now
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "organization_id"}, {Name: "member_id"}, {Name: "metric"}}, DoUpdates: clause.AssignmentColumns([]string{"allocated", "version", "active", "window_start", "window_end", "updated_at"})}).Create(&row).Error; err != nil {
			return mapError(err)
		}
		operation = operationRow{IdempotencyKey: input.IdempotencyKey, OrganizationID: quota.OrganizationID, MemberID: input.MemberID, Fingerprint: fingerprint, Target: input.Target, Version: row.Version, Allocated: row.Allocated, Consumed: consumed, Active: row.Active, WindowStart: quota.WindowStart, WindowEnd: quota.WindowEnd, CreatedAt: now}
		if err := tx.Create(&operation).Error; err != nil {
			return mapError(err)
		}
		if err := tx.Create(&auditRow{OrganizationID: quota.OrganizationID, ActorID: input.ActorID, MemberID: input.MemberID, Operation: operationName(input.Target), Target: input.Target, Allocated: row.Allocated, Consumed: consumed, Version: row.Version, IdempotencyKey: input.IdempotencyKey, CreatedAt: now}).Error; err != nil {
			return mapError(err)
		}
		result = allocationFromOperation(operation)
		return nil
	})
	return result, err
}

func (r *Repository) RevokeMissingMembers(ctx context.Context, quota domain.Quota, activeMemberIDs []string, actorID string) error {
	if r == nil || r.db == nil || !validQuota(quota) || actorID == "" {
		return domain.ErrInvalidRequest
	}
	active := make(map[string]struct{}, len(activeMemberIDs))
	for _, id := range activeMemberIDs {
		if id != "" {
			active[id] = struct{}{}
		}
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockOrganization(tx, quota.OrganizationID); err != nil {
			return err
		}
		var rows []allocationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND metric = ? AND window_start = ? AND window_end = ? AND active = ?", quota.OrganizationID, domain.MetricToken, quota.WindowStart, quota.WindowEnd, true).Find(&rows).Error; err != nil {
			return mapError(err)
		}
		for _, row := range rows {
			if _, ok := active[row.MemberID]; ok {
				continue
			}
			consumed, err := sumMemberConsumed(tx, quota, row.MemberID)
			if err != nil {
				return err
			}
			row.Allocated, row.Active, row.Version, row.UpdatedAt = consumed, false, row.Version+1, time.Now().UTC().Truncate(time.Microsecond)
			if err := tx.Save(&row).Error; err != nil {
				return mapError(err)
			}
			key := fmt.Sprintf("member-removed:%s:%s:%s", quota.OrganizationID, row.MemberID, listingsubscription.UsagePeriodKeyForWindow(quota.WindowStart, quota.WindowEnd))
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&auditRow{OrganizationID: quota.OrganizationID, ActorID: actorID, MemberID: row.MemberID, Operation: "revoke_member_removed", Target: 0, Allocated: consumed, Consumed: consumed, Version: row.Version, IdempotencyKey: key, CreatedAt: row.UpdatedAt}).Error; err != nil {
				return mapError(err)
			}
		}
		return nil
	})
}

func (r *Repository) Consume(ctx context.Context, quota domain.Quota, input domain.ConsumeInput) error {
	if r == nil || r.db == nil || !validQuota(quota) || input.OrganizationID != quota.OrganizationID || input.MemberID == "" || input.Quantity <= 0 {
		return domain.ErrInvalidRequest
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockOrganization(tx, quota.OrganizationID); err != nil {
			return err
		}
		var existing commercialUsageEventRow
		err := tx.Where("tenant_id = ? AND idempotency_key = ?", input.OrganizationID, input.IdempotencyKey).Take(&existing).Error
		if err == nil {
			if existing.MemberID != input.MemberID || existing.Metric != listingsubscription.UsageMetricAITokens || existing.Quantity != input.Quantity || existing.SourceType != input.SourceType || existing.SourceID != input.SourceID || existing.PeriodKey != listingsubscription.UsagePeriodKeyForWindow(quota.WindowStart, quota.WindowEnd) {
				return domain.ErrIdempotencyConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return mapError(err)
		}
		var row allocationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ? AND metric = ? AND window_start = ? AND window_end = ?", quota.OrganizationID, input.MemberID, domain.MetricToken, quota.WindowStart, quota.WindowEnd).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.ErrAllocationRequired
		} else if err != nil {
			return mapError(err)
		}
		if !row.Active || row.WindowStart.UTC() != quota.WindowStart.UTC() || row.WindowEnd.UTC() != quota.WindowEnd.UTC() {
			return domain.ErrAllocationRequired
		}
		memberConsumed, err := sumMemberConsumed(tx, quota, input.MemberID)
		if err != nil {
			return err
		}
		enterpriseConsumed, err := sumUsage(tx, quota)
		if err != nil {
			return err
		}
		if input.Quantity > row.Allocated-memberConsumed {
			return domain.ErrQuotaExceeded
		}
		if input.Quantity > quota.Total-enterpriseConsumed {
			return domain.ErrQuotaExceeded
		}
		// The usage event and bucket are committed by the commercial owner on
		// this same transaction. Allocation never creates a second usage fact.
		if _, err := r.commercial.SettleUsageInTransaction(ctx, tx, listingsubscription.ReserveUsageInput{
			TenantID: input.OrganizationID, ModuleCode: listingsubscription.ModuleListingKit, Metric: listingsubscription.UsageMetricAITokens,
			Quantity: input.Quantity, PeriodKey: listingsubscription.UsagePeriodKeyForWindow(quota.WindowStart, quota.WindowEnd),
			SourceType: input.SourceType, SourceID: input.SourceID, MemberID: input.MemberID, IdempotencyKey: input.IdempotencyKey, OccurredAt: quota.WindowStart.Add(time.Nanosecond),
		}); err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) lockOrganization(tx *gorm.DB, organizationID string) error {
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&lockRow{OrganizationID: organizationID, UpdatedAt: time.Now().UTC()}).Error; err != nil {
		return mapError(err)
	}
	var row lockRow
	return mapError(tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ?", organizationID).Take(&row).Error)
}

func (r *Repository) memberConsumed(ctx context.Context, quota domain.Quota, memberID string) (int64, error) {
	return r.commercial.SumCommittedMemberUsage(ctx, r.db, quota.OrganizationID, memberID, listingsubscription.UsageMetricAITokens, quota.WindowStart, quota.WindowEnd)
}

func sumMemberConsumed(tx *gorm.DB, quota domain.Quota, memberID string) (int64, error) {
	var total int64
	if err := tx.Model(&commercialUsageEventRow{}).Where("tenant_id = ? AND member_id = ? AND metric = ? AND status = ? AND occurred_at >= ? AND occurred_at < ?", quota.OrganizationID, memberID, listingsubscription.UsageMetricAITokens, string(listingsubscription.UsageEventCommitted), quota.WindowStart, quota.WindowEnd).Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error; err != nil {
		return 0, mapError(err)
	}
	return total, nil
}
func sumUsage(tx *gorm.DB, quota domain.Quota) (int64, error) {
	var total int64
	if err := tx.Model(&commercialUsageEventRow{}).Where("tenant_id = ? AND metric = ? AND status = ? AND occurred_at >= ? AND occurred_at < ?", quota.OrganizationID, listingsubscription.UsageMetricAITokens, string(listingsubscription.UsageEventCommitted), quota.WindowStart, quota.WindowEnd).Select("COALESCE(SUM(quantity), 0)").Scan(&total).Error; err != nil {
		return 0, mapError(err)
	}
	return total, nil
}

func (r *Repository) sumUsage(ctx context.Context, tx *gorm.DB, quota domain.Quota) (int64, error) {
	return sumUsage(tx.WithContext(ctx), quota)
}

func validQuota(q domain.Quota) bool {
	return q.OrganizationID != "" && q.Metric == domain.MetricToken && q.Total >= 0 && !q.WindowStart.IsZero() && q.WindowEnd.After(q.WindowStart)
}
func checkedAdd(a, b int64) (int64, error) {
	if b > 0 && a > math.MaxInt64-b {
		return 0, domain.ErrUnavailable
	}
	return a + b, nil
}
func setFingerprint(input domain.SetTargetInput) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%d|%s", input.OrganizationID, input.MemberID, input.Target, input.ExpectedVersion, input.ActorID)))
	return hex.EncodeToString(sum[:])
}
func allocationFromRow(row allocationRow, consumed int64) domain.Allocation {
	remaining := row.Allocated - consumed
	if remaining < 0 {
		remaining = 0
	}
	return domain.Allocation{OrganizationID: row.OrganizationID, MemberID: row.MemberID, Metric: row.Metric, Allocated: row.Allocated, Consumed: consumed, Remaining: remaining, Version: row.Version, Active: row.Active, WindowStart: row.WindowStart.UTC(), WindowEnd: row.WindowEnd.UTC()}
}
func allocationFromOperation(row operationRow) domain.Allocation {
	remaining := row.Allocated - row.Consumed
	if remaining < 0 {
		remaining = 0
	}
	return domain.Allocation{OrganizationID: row.OrganizationID, MemberID: row.MemberID, Metric: domain.MetricToken, Allocated: row.Allocated, Consumed: row.Consumed, Remaining: remaining, Version: row.Version, Active: row.Active, WindowStart: row.WindowStart.UTC(), WindowEnd: row.WindowEnd.UTC()}
}
func operationName(target int64) string {
	if target == 0 {
		return "revoke"
	}
	return "set_target"
}
func mapError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotFound
	}
	if strings.Contains(strings.ToLower(err.Error()), "unique") {
		return domain.ErrConflict
	}
	return err
}
