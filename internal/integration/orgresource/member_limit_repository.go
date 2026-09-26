package orgresourceadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"task-processor/internal/ledger/orgresource"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormMemberLimitRepository struct {
	db     *gorm.DB
	runner *transactionRunner
	now    func() time.Time
}

func NewGormMemberLimitRepository(db *gorm.DB, config TransactionConfig) (*GormMemberLimitRepository, error) {
	if db == nil {
		return nil, orgresource.ErrInvalidInput
	}
	return &GormMemberLimitRepository{db: db, runner: newTransactionRunner(db, config), now: time.Now}, nil
}
func (r *GormMemberLimitRepository) SetMonthlyLimit(ctx context.Context, input orgresource.SetMemberLimitExecution) (orgresource.MemberLimitSnapshot, error) {
	if !orgresource.ValidMemberLimitCommand(input) {
		return orgresource.MemberLimitSnapshot{}, orgresource.ErrInvalidInput
	}
	encoded, _ := json.Marshal(input)
	hash := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(hash[:])
	var result orgresource.MemberLimitSnapshot
	err := r.runner.run(ctx, func(tx *gorm.DB) error {
		op, err := lockFixedResourceOperation(tx, input.OrganizationID, input.OperationID, "set_member_ai_point_limit", fingerprint)
		if err != nil {
			return err
		}
		// Read the original receipt BEFORE calculating a new month. A retry in
		// October must not re-apply a September administration operation.
		if op.State == "succeeded" {
			return json.Unmarshal([]byte(op.ImmutableResult), &result)
		}
		if op.State != "processing" {
			return orgresource.ErrIdempotencyKeyConflict
		}
		now := r.now().UTC()
		month := orgresource.AIPointMonthStart(now)
		initial := memberAIPointLimitRow{OrganizationID: input.OrganizationID, MemberID: input.MemberID, UpdatedBy: input.ActorID, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; err != nil {
			return err
		}
		var config memberAIPointLimitRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ?", input.OrganizationID, input.MemberID).Take(&config).Error; err != nil {
			return err
		}
		if config.Version != input.ExpectedVersion || config.Version == math.MaxInt64 {
			return orgresource.ErrMemberLimitVersionConflict
		}
		counter, err := lockMemberMonth(tx, input.OrganizationID, input.MemberID, month)
		if err != nil {
			return err
		}
		if counter.Consumed > input.Target || counter.Reserved > input.Target-counter.Consumed {
			return orgresource.ErrMemberLimitExceeded
		}
		config.MonthlyLimit = input.Target
		config.Version++
		config.UpdatedBy = input.ActorID
		config.UpdatedAt = now
		if err := tx.Save(&config).Error; err != nil {
			return err
		}
		result = limitSnapshot(config, counter)
		payload, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Model(&organizationResourceOperationRow{}).Where("organization_id = ? AND operation_id = ?", input.OrganizationID, input.OperationID).Updates(map[string]any{"state": "succeeded", "immutable_result_snapshot": string(payload), "completed_at": now}).Error; err != nil {
			return err
		}
		return tx.Create(&organizationResourceAuditLogRow{OrganizationID: input.OrganizationID, OperationID: input.OperationID, Action: "set_member_ai_point_limit", ActorID: input.ActorID, Payload: string(payload), CreatedAt: now}).Error
	})
	if err != nil {
		return orgresource.MemberLimitSnapshot{}, err
	}
	return result, nil
}
func (r *GormMemberLimitRepository) ReadMonthlyLimit(ctx context.Context, org, member string) (orgresource.MemberLimitSnapshot, error) {
	var result orgresource.MemberLimitSnapshot
	err := r.runner.run(ctx, func(tx *gorm.DB) error {
		var config memberAIPointLimitRow
		if err := tx.Where("organization_id = ? AND member_id = ?", org, member).Take(&config).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.ErrMemberLimitUnavailable
		} else if err != nil {
			return err
		}
		counter := memberAIPointMonthRow{OrganizationID: org, MemberID: member, MonthStart: orgresource.AIPointMonthStart(r.now())}
		err := tx.Where("organization_id = ? AND member_id = ? AND month_start = ?", org, member, counter.MonthStart).Take(&counter).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result = limitSnapshot(config, counter)
		return nil
	})
	if err != nil {
		return orgresource.MemberLimitSnapshot{}, err
	}
	return result, nil
}

func lockFixedResourceOperation(tx *gorm.DB, org, key, kind, fingerprint string) (organizationResourceOperationRow, error) {
	row := organizationResourceOperationRow{OrganizationID: org, OperationID: key, OperationType: kind, RequestFingerprint: fingerprint, State: "processing"}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return row, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND operation_id = ?", org, key).Take(&row).Error; err != nil {
		return row, err
	}
	if row.OperationType != kind || row.RequestFingerprint != fingerprint {
		return row, orgresource.ErrIdempotencyKeyConflict
	}
	return row, nil
}
func lockMemberMonth(tx *gorm.DB, org, member string, month time.Time) (memberAIPointMonthRow, error) {
	row := memberAIPointMonthRow{OrganizationID: org, MemberID: member, MonthStart: month}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return row, err
	}
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ? AND month_start = ?", org, member, month).Take(&row).Error
	return row, err
}
func limitSnapshot(config memberAIPointLimitRow, counter memberAIPointMonthRow) orgresource.MemberLimitSnapshot {
	return orgresource.MemberLimitSnapshot{OrganizationID: config.OrganizationID, MemberID: config.MemberID, MonthlyLimit: config.MonthlyLimit, Version: config.Version, MonthStart: counter.MonthStart, Reserved: counter.Reserved, Consumed: counter.Consumed}
}
