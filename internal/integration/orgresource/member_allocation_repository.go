package orgresourceadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"task-processor/internal/ledger/orgresource"
)

type GormMemberAllocationRepository struct {
	db     *gorm.DB
	runner *transactionRunner
	now    func() time.Time
}

func NewGormMemberAllocationRepository(db *gorm.DB, config TransactionConfig) (*GormMemberAllocationRepository, error) {
	if db == nil {
		return nil, orgresource.ErrInvalidInput
	}
	return &GormMemberAllocationRepository{db: db, runner: newTransactionRunner(db, config), now: time.Now}, nil
}
func (r *GormMemberAllocationRepository) Transfer(ctx context.Context, c orgresource.MemberResourceTransfer) (orgresource.MemberResourceTransferResult, error) {
	if !orgresource.ValidMemberResourceTransfer(c) {
		return orgresource.MemberResourceTransferResult{}, orgresource.ErrInvalidInput
	}
	encoded, err := json.Marshal(c)
	if err != nil {
		return orgresource.MemberResourceTransferResult{}, err
	}
	hash := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(hash[:])
	var result orgresource.MemberResourceTransferResult
	err = r.runner.run(ctx, func(tx *gorm.DB) error {
		var bucket organizationResourceBucketRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", c.OrganizationID, c.ResourceType).Take(&bucket).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.ErrInsufficientBalance
		} else if err != nil {
			return err
		}
		position, err := lockMemberResourcePosition(tx, c.OrganizationID, c.MemberID, c.ResourceType)
		if err != nil {
			return err
		}
		op, err := lockFixedResourceOperation(tx, c.OrganizationID, c.OperationID, string(c.Action), fingerprint)
		if err != nil {
			return err
		}
		// Replay the immutable receipt before checking current version or debt.
		if op.State == "succeeded" {
			if err := json.Unmarshal([]byte(op.ImmutableResult), &result); err != nil {
				return err
			}
			result.Replayed = true
			return nil
		}
		if op.State != "processing" {
			return orgresource.ErrIdempotencyKeyConflict
		}
		if position.Version != c.ExpectedVersion || position.Version == math.MaxInt64 {
			return orgresource.ErrMemberResourceVersionConflict
		}
		now := r.now().UTC()
		availableDelta, allocatedDelta := -c.Quantity, c.Quantity
		if c.Action == orgresource.MemberResourceAllocate {
			var debt organizationResourceDebtRow
			if err := tx.Where("organization_id = ? AND resource_type = ?", c.OrganizationID, c.ResourceType).Take(&debt).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if debt.Amount > 0 {
				return orgresource.ErrResourceDebtOutstanding
			}
			if bucket.Available < c.Quantity {
				return orgresource.ErrInsufficientBalance
			}
			if position.Free > math.MaxInt64-c.Quantity || bucket.Allocated > math.MaxInt64-c.Quantity {
				return orgresource.ErrInvalidInput
			}
			bucket.Available -= c.Quantity
			bucket.Allocated += c.Quantity
			position.Free += c.Quantity
		} else {
			if position.Free < c.Quantity {
				return orgresource.ErrInsufficientBalance
			}
			if bucket.Allocated < c.Quantity {
				return errors.New("member free position exceeds allocated bucket")
			}
			credit, err := applyPositiveCredit(ctx, tx, c.OrganizationID, string(c.ResourceType), c.Quantity, bucket.Available, now)
			if err != nil {
				return err
			}
			result.GrossCredit, result.DebtRepaid, result.NetCredit = credit.gross, credit.debtRepaid, credit.net
			bucket.Available = credit.availableAfter
			bucket.Allocated -= c.Quantity
			position.Free -= c.Quantity
			availableDelta, allocatedDelta = credit.net, -c.Quantity
		}
		position.Version++
		position.UpdatedAt = now
		if err := tx.Save(&position).Error; err != nil {
			return err
		}
		if err := tx.Model(&organizationResourceBucketRow{}).Where("organization_id = ? AND resource_type = ?", c.OrganizationID, c.ResourceType).Updates(map[string]any{"available": bucket.Available, "allocated": bucket.Allocated, "updated_at": now}).Error; err != nil {
			return err
		}
		result.Position, result.Unallocated, result.Allocated = memberResourcePosition(position), bucket.Available, bucket.Allocated
		payload, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Model(&organizationResourceOperationRow{}).Where("organization_id = ? AND operation_id = ?", c.OrganizationID, c.OperationID).Updates(map[string]any{"state": "succeeded", "immutable_result_snapshot": string(payload), "completed_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Create(&organizationResourceEventRow{EventID: uuid.NewString(), OrganizationID: c.OrganizationID, OperationID: c.OperationID, ResourceType: string(c.ResourceType), Quantity: c.Quantity, AvailableDelta: availableDelta, AllocatedDelta: allocatedDelta, Reason: string(c.Action), SourceType: "member_resource_position", SourceIdentity: c.MemberID, BalanceAfter: bucket.Available, AvailableAfter: bucket.Available, AllocatedAfter: bucket.Allocated, ReservedAfter: bucket.Reserved, ConsumedAfter: bucket.Consumed, GrossCredit: result.GrossCredit, DebtRepaid: result.DebtRepaid, NetCredit: result.NetCredit}).Error; err != nil {
			return err
		}
		return tx.Create(&organizationResourceAuditLogRow{OrganizationID: c.OrganizationID, OperationID: c.OperationID, Action: string(c.Action), ActorID: c.ActorID, Payload: string(payload), CreatedAt: now}).Error
	})
	if err != nil {
		return orgresource.MemberResourceTransferResult{}, err
	}
	return result, nil
}
func (r *GormMemberAllocationRepository) ReadPosition(ctx context.Context, org, member string, resource orgresource.ResourceType) (orgresource.MemberResourcePosition, error) {
	if !orgresource.ValidMemberResourcePositionIdentity(org, member, resource) {
		return orgresource.MemberResourcePosition{}, orgresource.ErrInvalidInput
	}
	row := memberResourcePositionRow{OrganizationID: org, MemberID: member, ResourceType: string(resource)}
	err := r.db.WithContext(ctx).Where("organization_id = ? AND member_id = ? AND resource_type = ?", org, member, resource).Take(&row).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.MemberResourcePosition{}, err
	}
	return memberResourcePosition(row), nil
}
func lockMemberResourcePosition(tx *gorm.DB, org, member string, resource orgresource.ResourceType) (memberResourcePositionRow, error) {
	row := memberResourcePositionRow{OrganizationID: org, MemberID: member, ResourceType: string(resource)}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return row, err
	}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ? AND resource_type = ?", org, member, resource).Take(&row).Error
	return row, err
}
func memberResourcePosition(row memberResourcePositionRow) orgresource.MemberResourcePosition {
	return orgresource.MemberResourcePosition{OrganizationID: row.OrganizationID, MemberID: row.MemberID, ResourceType: orgresource.ResourceType(row.ResourceType), Free: row.Free, Reserved: row.Reserved, Consumed: row.Consumed, Version: row.Version, UpdatedAt: row.UpdatedAt}
}
