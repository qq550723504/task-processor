package listingsubscription

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type storeQuotaAllocationRow struct {
	AllocationID   string     `gorm:"column:allocation_id;primaryKey;size:36"`
	OrganizationID string     `gorm:"column:organization_id;not null;size:200;uniqueIndex:idx_saas_store_quota_org_request,priority:1;uniqueIndex:idx_saas_store_quota_org_store,priority:1;index:idx_saas_store_quota_org_status,priority:1"`
	StoreID        string     `gorm:"column:store_id;not null;size:36;uniqueIndex:idx_saas_store_quota_org_store,priority:2"`
	RequestKey     string     `gorm:"column:request_key;not null;size:36;uniqueIndex:idx_saas_store_quota_org_request,priority:2"`
	Status         string     `gorm:"column:status;not null;size:16;check:status IN ('reserved','allocated','released');index:idx_saas_store_quota_org_status,priority:2"`
	CreatedBy      string     `gorm:"column:created_by;not null;size:200"`
	UpdatedBy      string     `gorm:"column:updated_by;not null;size:200"`
	AllocatedAt    *time.Time `gorm:"column:allocated_at"`
	ReleasedAt     *time.Time `gorm:"column:released_at"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (storeQuotaAllocationRow) TableName() string { return "saas_store_quota_allocations" }

type storeQuotaBucketRow struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey;size:200"`
	Committed      int64     `gorm:"column:committed;not null;default:0;check:committed >= 0"`
	Reserved       int64     `gorm:"column:reserved;not null;default:0;check:reserved >= 0"`
	Version        int64     `gorm:"column:version;not null;default:1;check:version > 0"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (storeQuotaBucketRow) TableName() string { return "saas_store_quota_buckets" }

type gormStoreQuotaLedger struct {
	repo *GormRepository
	now  func() time.Time
}

func NewGormStoreQuotaLedger(repo *GormRepository) StoreQuotaLedger {
	return newGormStoreQuotaLedger(repo, time.Now)
}

func newGormStoreQuotaLedger(repo *GormRepository, now func() time.Time) *gormStoreQuotaLedger {
	return &gormStoreQuotaLedger{repo: repo, now: now}
}

func (l *gormStoreQuotaLedger) Reserve(ctx context.Context, input StoreQuotaReserveInput) (StoreQuotaReserveResult, error) {
	input, err := NormalizeAndValidateStoreQuotaReserveInput(input)
	if err != nil {
		return StoreQuotaReserveResult{}, err
	}
	if existing, err := l.GetByRequestKey(ctx, input.OrganizationID, input.RequestKey); err == nil {
		if existing.CreatedBy != input.ActorSubject {
			return StoreQuotaReserveResult{}, ErrStoreQuotaIdentityMismatch
		}
		return storeQuotaReserveResult(*existing, true), nil
	} else if !errors.Is(err, ErrStoreQuotaNotFound) {
		return StoreQuotaReserveResult{}, err
	}

	var result StoreQuotaReserveResult
	err = l.withRetry(ctx, func(tx *gorm.DB) error {
		var existing storeQuotaAllocationRow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND request_key = ?", input.OrganizationID, input.RequestKey).Take(&existing).Error
		if err == nil {
			allocation := storeQuotaAllocationFromRow(existing)
			if allocation.CreatedBy != input.ActorSubject {
				return ErrStoreQuotaIdentityMismatch
			}
			result = storeQuotaReserveResult(allocation, true)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		limit, err := storeQuotaLimit(tx, input.OrganizationID, l.now().UTC())
		if err != nil {
			return err
		}
		bucket, err := loadOrCreateStoreQuotaBucket(tx, input.OrganizationID)
		if err != nil {
			return err
		}
		if err := validateStoreQuotaBucket(bucket); err != nil {
			return err
		}
		if bucket.Committed > math.MaxInt64-bucket.Reserved || bucket.Committed+bucket.Reserved >= limit {
			return &StoreQuotaExceededError{OrganizationID: input.OrganizationID, Committed: bucket.Committed, Reserved: bucket.Reserved, Limit: limit}
		}
		now := l.now().UTC()
		row := storeQuotaAllocationRow{AllocationID: uuid.NewString(), OrganizationID: input.OrganizationID, StoreID: uuid.NewString(), RequestKey: input.RequestKey, Status: string(StoreQuotaReserved), CreatedBy: input.ActorSubject, UpdatedBy: input.ActorSubject, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := updateStoreQuotaBucket(tx, bucket, bucket.Committed, bucket.Reserved+1, now); err != nil {
			return err
		}
		result = storeQuotaReserveResult(storeQuotaAllocationFromRow(row), false)
		return nil
	})
	return result, err
}

func (l *gormStoreQuotaLedger) Commit(ctx context.Context, input StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error) {
	return l.transition(ctx, input, StoreQuotaAllocated, storeQuotaCommit)
}

func (l *gormStoreQuotaLedger) ReleaseReservation(ctx context.Context, input StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error) {
	return l.transition(ctx, input, StoreQuotaReleased, storeQuotaReleaseReservation)
}

func (l *gormStoreQuotaLedger) Deallocate(ctx context.Context, input StoreQuotaTransitionInput) (StoreQuotaTransitionResult, error) {
	return l.transition(ctx, input, StoreQuotaReleased, storeQuotaDeallocate)
}

type storeQuotaOperation string

const (
	storeQuotaCommit             storeQuotaOperation = "commit"
	storeQuotaReleaseReservation storeQuotaOperation = "release_reservation"
	storeQuotaDeallocate         storeQuotaOperation = "deallocate"
)

func (l *gormStoreQuotaLedger) transition(ctx context.Context, input StoreQuotaTransitionInput, target StoreQuotaAllocationStatus, operation storeQuotaOperation) (StoreQuotaTransitionResult, error) {
	input, err := normalizeAndValidateStoreQuotaTransitionInput(input)
	if err != nil {
		return StoreQuotaTransitionResult{}, err
	}
	var result StoreQuotaTransitionResult
	err = l.withRetry(ctx, func(tx *gorm.DB) error {
		var row storeQuotaAllocationRow
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND allocation_id = ?", input.OrganizationID, input.AllocationID).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrStoreQuotaNotFound
		}
		if err != nil {
			return err
		}
		if row.StoreID != input.StoreID || row.RequestKey != input.RequestKey || row.CreatedBy != input.ActorSubject {
			return ErrStoreQuotaIdentityMismatch
		}
		allocation := storeQuotaAllocationFromRow(row)
		if allocation.Status == StoreQuotaReleased {
			if (operation == storeQuotaReleaseReservation && row.AllocatedAt == nil && row.ReleasedAt != nil) || (operation == storeQuotaDeallocate && row.AllocatedAt != nil && row.ReleasedAt != nil) {
				result = StoreQuotaTransitionResult{Allocation: allocation, Existing: true}
				return nil
			}
			return ErrStoreQuotaInvalidTransition
		}
		if allocation.Status == StoreQuotaAllocated {
			if target == StoreQuotaAllocated {
				result = StoreQuotaTransitionResult{Allocation: allocation, Existing: true}
				return nil
			}
			if operation != storeQuotaDeallocate {
				return ErrStoreQuotaInvalidTransition
			}
			bucket, err := loadStoreQuotaBucket(tx, input.OrganizationID)
			if err != nil {
				return err
			}
			if err := validateStoreQuotaBucket(bucket); err != nil {
				return err
			}
			if bucket.Committed < 1 {
				return ErrStoreQuotaInvalidTransition
			}
			now := l.now().UTC()
			if err := updateStoreQuotaBucket(tx, bucket, bucket.Committed-1, bucket.Reserved, now); err != nil {
				return err
			}
			if err := updateStoreQuotaAllocation(tx, row, StoreQuotaReleased, input.ActorSubject, nil, &now, now); err != nil {
				return err
			}
			row.Status, row.UpdatedBy, row.UpdatedAt, row.ReleasedAt = string(StoreQuotaReleased), input.ActorSubject, now, &now
			result = StoreQuotaTransitionResult{Allocation: storeQuotaAllocationFromRow(row)}
			return nil
		}
		if allocation.Status != StoreQuotaReserved {
			return ErrStoreQuotaInvalidTransition
		}
		bucket, err := loadStoreQuotaBucket(tx, input.OrganizationID)
		if err != nil {
			return err
		}
		if err := validateStoreQuotaBucket(bucket); err != nil {
			return err
		}
		if bucket.Reserved < 1 {
			return ErrStoreQuotaInvalidTransition
		}
		now := l.now().UTC()
		if operation == storeQuotaCommit {
			if err := updateStoreQuotaBucket(tx, bucket, bucket.Committed+1, bucket.Reserved-1, now); err != nil {
				return err
			}
			if err := updateStoreQuotaAllocation(tx, row, StoreQuotaAllocated, input.ActorSubject, &now, nil, now); err != nil {
				return err
			}
			row.Status, row.UpdatedBy, row.UpdatedAt, row.AllocatedAt = string(StoreQuotaAllocated), input.ActorSubject, now, &now
		} else if operation == storeQuotaReleaseReservation {
			if err := updateStoreQuotaBucket(tx, bucket, bucket.Committed, bucket.Reserved-1, now); err != nil {
				return err
			}
			if err := updateStoreQuotaAllocation(tx, row, StoreQuotaReleased, input.ActorSubject, nil, &now, now); err != nil {
				return err
			}
			row.Status, row.UpdatedBy, row.UpdatedAt, row.ReleasedAt = string(StoreQuotaReleased), input.ActorSubject, now, &now
		} else {
			return ErrStoreQuotaInvalidTransition
		}
		result = StoreQuotaTransitionResult{Allocation: storeQuotaAllocationFromRow(row)}
		return nil
	})
	return result, err
}

func (l *gormStoreQuotaLedger) GetByRequestKey(ctx context.Context, organizationID, requestKey string) (*StoreQuotaAllocation, error) {
	if err := validateStoreQuotaText(organizationID, "organization_id"); err != nil {
		return nil, err
	}
	if err := validateStoreQuotaUUID(requestKey, "request_key"); err != nil {
		return nil, err
	}
	var row storeQuotaAllocationRow
	err := l.repo.db.WithContext(ctx).Where("organization_id = ? AND request_key = ?", organizationID, requestKey).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrStoreQuotaNotFound
	}
	if err != nil {
		return nil, err
	}
	allocation := storeQuotaAllocationFromRow(row)
	return &allocation, nil
}

func (l *gormStoreQuotaLedger) Summary(ctx context.Context, organizationID string) (StoreQuotaSummary, error) {
	if err := validateStoreQuotaText(organizationID, "organization_id"); err != nil {
		return StoreQuotaSummary{}, err
	}
	result := StoreQuotaSummary{OrganizationID: organizationID}
	err := l.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bucket storeQuotaBucketRow
		err := tx.Where("organization_id = ?", organizationID).Take(&bucket).Error
		if err == nil {
			if err := validateStoreQuotaBucket(bucket); err != nil {
				return err
			}
			result.Committed, result.Reserved = bucket.Committed, bucket.Reserved
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		limit, err := storeQuotaLimit(tx, organizationID, l.now().UTC())
		if errors.Is(err, ErrSubscriptionRequired) {
			result.Allowed, result.Reason = false, "subscription_required"
			return nil
		}
		if err != nil {
			return err
		}
		result.Limit = &limit
		result.Allowed = result.Committed < limit && result.Reserved < limit-result.Committed
		if !result.Allowed {
			result.Reason = "store_limit_reached"
		}
		return nil
	})
	return result, err
}

func (l *gormStoreQuotaLedger) withRetry(ctx context.Context, operation func(*gorm.DB) error) error {
	if l == nil || l.repo == nil || l.repo.db == nil {
		return errors.New("store quota repository is not configured")
	}
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		err = l.repo.db.WithContext(ctx).Transaction(operation)
		if !isRetryableUsageLedgerError(err) && !errors.Is(err, errStoreQuotaVersionRace) {
			return err
		}
	}
	return err
}

var errStoreQuotaVersionRace = errors.New("store quota bucket version race")

func loadStoreQuotaBucket(tx *gorm.DB, organizationID string) (storeQuotaBucketRow, error) {
	var bucket storeQuotaBucketRow
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ?", organizationID).Take(&bucket).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return storeQuotaBucketRow{}, ErrStoreQuotaInvalidTransition
	}
	return bucket, err
}

func loadOrCreateStoreQuotaBucket(tx *gorm.DB, organizationID string) (storeQuotaBucketRow, error) {
	bucket, err := loadStoreQuotaBucket(tx, organizationID)
	if err == nil {
		return bucket, nil
	}
	if !errors.Is(err, ErrStoreQuotaInvalidTransition) {
		return storeQuotaBucketRow{}, err
	}
	bucket = storeQuotaBucketRow{OrganizationID: organizationID, Version: 1}
	if err := tx.Create(&bucket).Error; err != nil {
		return storeQuotaBucketRow{}, err
	}
	return bucket, nil
}

func validateStoreQuotaBucket(bucket storeQuotaBucketRow) error {
	if bucket.Committed < 0 || bucket.Reserved < 0 || bucket.Version < 1 || bucket.Committed > math.MaxInt64-bucket.Reserved {
		return ErrStoreQuotaInvalidTransition
	}
	return nil
}

func updateStoreQuotaBucket(tx *gorm.DB, bucket storeQuotaBucketRow, committed, reserved int64, now time.Time) error {
	if committed < 0 || reserved < 0 || committed > math.MaxInt64-reserved {
		return ErrStoreQuotaInvalidTransition
	}
	result := tx.Model(&storeQuotaBucketRow{}).Where("organization_id = ? AND version = ?", bucket.OrganizationID, bucket.Version).Updates(map[string]any{"committed": committed, "reserved": reserved, "version": bucket.Version + 1, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errStoreQuotaVersionRace
	}
	return nil
}

func updateStoreQuotaAllocation(tx *gorm.DB, row storeQuotaAllocationRow, status StoreQuotaAllocationStatus, actor string, allocatedAt, releasedAt *time.Time, updatedAt time.Time) error {
	if allocatedAt == nil {
		allocatedAt = row.AllocatedAt
	}
	if releasedAt == nil {
		releasedAt = row.ReleasedAt
	}
	result := tx.Model(&storeQuotaAllocationRow{}).Where("organization_id = ? AND allocation_id = ? AND status = ?", row.OrganizationID, row.AllocationID, row.Status).Updates(map[string]any{"status": string(status), "updated_by": actor, "updated_at": updatedAt, "allocated_at": allocatedAt, "released_at": releasedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errStoreQuotaVersionRace
	}
	return nil
}

func storeQuotaLimit(tx *gorm.DB, organizationID string, now time.Time) (int64, error) {
	var entitlementRow tenantEntitlementRow
	if err := tx.Where("tenant_id = ? AND module_code = ?", organizationID, ModuleStoreManagement).Take(&entitlementRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrSubscriptionRequired
		}
		return 0, err
	}
	entitlement, err := entitlementRow.toEntitlement()
	if err != nil {
		return 0, err
	}
	if allowed, _ := evaluateEntitlement(entitlement, now); !allowed {
		return 0, ErrSubscriptionRequired
	}
	limits := entitlement.Limits
	if value, ok := limits[storeQuotaMetric]; ok {
		if value <= 0 {
			return 0, ErrSubscriptionRequired
		}
		return int64(value), nil
	}
	var subscription tenantSubscriptionRow
	if err := tx.Where("tenant_id = ?", organizationID).Take(&subscription).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrSubscriptionRequired
		}
		return 0, err
	}
	subscriptionValue := subscription.toTenantSubscription()
	subscriptionEntitlement := &Entitlement{Status: subscriptionValue.Status, StartsAt: subscriptionValue.StartsAt, ExpiresAt: subscriptionValue.ExpiresAt}
	if allowed, _ := evaluateEntitlement(subscriptionEntitlement, now); !allowed {
		return 0, ErrSubscriptionRequired
	}
	var module subscriptionPlanModuleRow
	if err := tx.Where("plan_code = ? AND module_code = ?", subscription.PlanCode, ModuleStoreManagement).Take(&module).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrSubscriptionRequired
		}
		return 0, err
	}
	planLimits, err := unmarshalLimits(module.LimitsJSON)
	if err != nil {
		return 0, err
	}
	value, ok := planLimits[storeQuotaMetric]
	if !ok || value <= 0 {
		return 0, ErrSubscriptionRequired
	}
	return int64(value), nil
}

func storeQuotaAllocationFromRow(row storeQuotaAllocationRow) StoreQuotaAllocation {
	return StoreQuotaAllocation{OrganizationID: row.OrganizationID, AllocationID: row.AllocationID, StoreID: row.StoreID, RequestKey: row.RequestKey, Status: StoreQuotaAllocationStatus(row.Status), CreatedBy: row.CreatedBy, UpdatedBy: row.UpdatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, AllocatedAt: cloneStoreQuotaTime(row.AllocatedAt), ReleasedAt: cloneStoreQuotaTime(row.ReleasedAt)}
}

func storeQuotaReserveResult(allocation StoreQuotaAllocation, existing bool) StoreQuotaReserveResult {
	return StoreQuotaReserveResult{Allocation: allocation, AllocationID: allocation.AllocationID, StoreID: allocation.StoreID, Existing: existing}
}

func cloneStoreQuotaTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
