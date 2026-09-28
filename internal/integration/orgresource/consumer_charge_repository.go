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

const consumerChargeProtocol = "consumer_charge_v1"

type GormConsumerChargeRepository struct {
	db     *gorm.DB
	runner *transactionRunner
	now    func() time.Time
}

func NewGormConsumerChargeRepository(db *gorm.DB, config TransactionConfig) (*GormConsumerChargeRepository, error) {
	if db == nil {
		return nil, orgresource.ErrInvalidInput
	}
	return &GormConsumerChargeRepository{db: db, runner: newTransactionRunner(db, config), now: time.Now}, nil
}
func chargeOperationKey(prefix string, identity orgresource.ConsumerChargeIdentity) string {
	value, _ := json.Marshal(identity)
	hash := sha256.Sum256(value)
	return prefix + hex.EncodeToString(hash[:])
}
func chargeFingerprint(intent orgresource.ConsumerChargeIntent) string {
	value, _ := json.Marshal(intent)
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}
func (r *GormConsumerChargeRepository) Reserve(ctx context.Context, intent orgresource.ConsumerChargeIntent) (orgresource.ConsumerChargeReceipt, error) {
	if !orgresource.ValidConsumerChargeIntent(intent) {
		return orgresource.ConsumerChargeReceipt{}, orgresource.ErrInvalidInput
	}
	var receipt orgresource.ConsumerChargeReceipt
	err := r.runner.run(ctx, func(tx *gorm.DB) error {
		var bucket organizationResourceBucketRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", intent.Identity.OrganizationID, intent.ResourceType).Take(&bucket).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.ErrInsufficientBalance
		} else if err != nil {
			return err
		}
		var position memberResourcePositionRow
		if intent.Funding == orgresource.FundingMember {
			var err error
			position, err = lockMemberResourcePosition(tx, intent.Identity.OrganizationID, intent.MemberID, intent.ResourceType)
			if err != nil {
				return err
			}
		}
		op, err := lockFixedResourceOperation(tx, intent.Identity.OrganizationID, chargeOperationKey("consumer-reserve:", intent.Identity), "reserve_consumer_resource", chargeFingerprint(intent))
		if err != nil {
			return err
		}
		if op.State == "succeeded" {
			row, err := readConsumerReservation(tx, intent.Identity, false)
			if err != nil {
				return err
			}
			receipt = consumerChargeReceipt(row)
			if receipt.Intent != intent {
				return orgresource.ErrIdempotencyKeyConflict
			}
			return nil
		}
		if op.State != "processing" {
			return orgresource.ErrIdempotencyKeyConflict
		}
		var debt organizationResourceDebtRow
		if err := tx.Where("organization_id = ? AND resource_type = ?", intent.Identity.OrganizationID, intent.ResourceType).Take(&debt).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if debt.Amount > 0 {
			return orgresource.ErrResourceDebtOutstanding
		}
		if bucket.Reserved > math.MaxInt64-intent.Quantity {
			return orgresource.ErrInvalidInput
		}
		now := r.now().UTC()
		availableDelta, allocatedDelta := int64(0), int64(0)
		if intent.Funding == orgresource.FundingMember {
			if position.Free < intent.Quantity {
				return orgresource.ErrInsufficientBalance
			}
			if bucket.Allocated < intent.Quantity {
				return errors.New("member free balance exceeds allocated bucket")
			}
			if position.Reserved > math.MaxInt64-intent.Quantity || position.Version == math.MaxInt64 {
				return orgresource.ErrInvalidInput
			}
			position.Free -= intent.Quantity
			position.Reserved += intent.Quantity
			position.Version++
			position.UpdatedAt = now
			if err := tx.Save(&position).Error; err != nil {
				return err
			}
			bucket.Allocated -= intent.Quantity
			allocatedDelta = -intent.Quantity
		} else {
			if bucket.Available < intent.Quantity {
				return orgresource.ErrInsufficientBalance
			}
			bucket.Available -= intent.Quantity
			availableDelta = -intent.Quantity
		}
		bucket.Reserved += intent.Quantity
		row := organizationResourceReservationRow{ChargeProtocol: consumerChargeProtocol, ChargeActorID: intent.ActorID, ChargeFunding: string(intent.Funding), NextCheckAt: &now, OrganizationID: intent.Identity.OrganizationID, ReservationID: uuid.NewString(), OperationID: op.OperationID, OwnerType: string(intent.Identity.Consumer), OwnerAttemptID: intent.Identity.OperationID, BusinessScope: intent.BusinessScope, ResourceType: string(intent.ResourceType), ReservationPurpose: consumerChargeProtocol, Quantity: intent.Quantity, State: string(orgresource.ReservationReserved), RequestFingerprint: intent.Fingerprint, MemberID: intent.MemberID, CreatedAt: now}
		if err := tx.Omit("Events").Create(&row).Error; err != nil {
			return err
		}
		if err := updateConsumerBucket(tx, bucket, now); err != nil {
			return err
		}
		receipt = consumerChargeReceipt(row)
		if err := completeChargeOperation(tx, op, receipt, now); err != nil {
			return err
		}
		return createConsumerEvent(tx, row, bucket, op.OperationID, "reserve", availableDelta, allocatedDelta, intent.Quantity, 0, positiveCreditAllocation{}, now)
	})
	if err != nil {
		if read, readErr := r.Read(ctx, intent.Identity); readErr == nil && read.Intent == intent {
			return read, nil
		}
		return orgresource.ConsumerChargeReceipt{}, err
	}
	return receipt, nil
}
func (r *GormConsumerChargeRepository) Read(ctx context.Context, identity orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	if !orgresource.ValidConsumerChargeIdentity(identity) {
		return orgresource.ConsumerChargeReceipt{}, orgresource.ErrInvalidInput
	}
	var receipt orgresource.ConsumerChargeReceipt
	err := r.runner.runRead(ctx, func(readContext context.Context) error {
		row, err := readConsumerReservation(r.db.WithContext(readContext), identity, false)
		if err != nil {
			return err
		}
		receipt = consumerChargeReceipt(row)
		return nil
	})
	return receipt, err
}
func (r *GormConsumerChargeRepository) Settle(ctx context.Context, original orgresource.ConsumerChargeReceipt, proof orgresource.ConsumerChargeProof) (orgresource.ConsumerChargeReceipt, error) {
	if !orgresource.ValidConsumerChargeIntent(original.Intent) || proof.Intent != original.Intent || proof.ReservationID != original.ReservationID {
		return orgresource.ConsumerChargeReceipt{}, orgresource.ErrIdempotencyKeyConflict
	}
	if (proof.State != orgresource.ConsumerEffectSucceeded && proof.State != orgresource.ConsumerEffectFailed) || proof.EvidenceID == "" || len(proof.EvidenceID) > 256 {
		return orgresource.ConsumerChargeReceipt{}, orgresource.ErrConsumerChargeUnknown
	}
	intent := original.Intent
	var receipt orgresource.ConsumerChargeReceipt
	err := r.runner.run(ctx, func(tx *gorm.DB) error {
		var bucket organizationResourceBucketRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", intent.Identity.OrganizationID, intent.ResourceType).Take(&bucket).Error; err != nil {
			return err
		}
		var position memberResourcePositionRow
		if intent.Funding == orgresource.FundingMember {
			var err error
			position, err = lockMemberResourcePosition(tx, intent.Identity.OrganizationID, intent.MemberID, intent.ResourceType)
			if err != nil {
				return err
			}
		}
		op, err := lockFixedResourceOperation(tx, intent.Identity.OrganizationID, chargeOperationKey("consumer-settle:", intent.Identity), "settle_consumer_resource", chargeFingerprint(intent))
		if err != nil {
			return err
		}
		row, err := readConsumerReservation(tx, intent.Identity, true)
		if err != nil {
			return err
		}
		receipt = consumerChargeReceipt(row)
		if receipt.Intent != intent || receipt.ReservationID != original.ReservationID {
			return orgresource.ErrIdempotencyKeyConflict
		}
		if op.State == "succeeded" {
			return nil
		}
		if receipt.State != orgresource.ReservationReserved {
			return orgresource.ErrIdempotencyKeyConflict
		}
		if bucket.Reserved < intent.Quantity {
			return errors.New("consumer reserved balance is inconsistent")
		}
		now := r.now().UTC()
		credit := positiveCreditAllocation{}
		availableDelta, allocatedDelta, consumedDelta := int64(0), int64(0), int64(0)
		bucket.Reserved -= intent.Quantity
		if proof.State == orgresource.ConsumerEffectSucceeded {
			if bucket.Consumed > math.MaxInt64-intent.Quantity {
				return orgresource.ErrInvalidInput
			}
			bucket.Consumed += intent.Quantity
			consumedDelta = intent.Quantity
			row.State = string(orgresource.ReservationCommitted)
		} else {
			creditBefore := bucket.Available
			if intent.Funding == orgresource.FundingMember {
				creditBefore = bucket.Allocated
			}
			credit, err = applyPositiveCredit(ctx, tx, intent.Identity.OrganizationID, string(intent.ResourceType), intent.Quantity, creditBefore, now)
			if err != nil {
				return err
			}
			if intent.Funding == orgresource.FundingMember {
				bucket.Allocated = credit.availableAfter
				allocatedDelta = credit.net
			} else {
				bucket.Available = credit.availableAfter
				availableDelta = credit.net
			}
			row.State = string(orgresource.ReservationReleased)
		}
		if intent.Funding == orgresource.FundingMember {
			if position.Reserved < intent.Quantity || position.Version == math.MaxInt64 {
				return errors.New("member reserved balance is inconsistent")
			}
			position.Reserved -= intent.Quantity
			position.Version++
			position.UpdatedAt = now
			if proof.State == orgresource.ConsumerEffectSucceeded {
				if position.Consumed > math.MaxInt64-intent.Quantity {
					return orgresource.ErrInvalidInput
				}
				position.Consumed += intent.Quantity
			} else {
				if position.Free > math.MaxInt64-credit.net {
					return orgresource.ErrInvalidInput
				}
				position.Free += credit.net
			}
			if err := tx.Save(&position).Error; err != nil {
				return err
			}
		}
		row.ChargeEvidenceID = proof.EvidenceID
		row.SettledAt = &now
		row.SettlementOperationID = &op.OperationID
		row.NextCheckAt = nil
		if err := tx.Omit("Events").Save(&row).Error; err != nil {
			return err
		}
		if err := updateConsumerBucket(tx, bucket, now); err != nil {
			return err
		}
		receipt = consumerChargeReceipt(row)
		if err := completeChargeOperation(tx, op, receipt, now); err != nil {
			return err
		}
		return createConsumerEvent(tx, row, bucket, op.OperationID, string(row.State), availableDelta, allocatedDelta, -intent.Quantity, consumedDelta, credit, now)
	})
	if err != nil {
		if read, readErr := r.Read(ctx, intent.Identity); readErr == nil && read.Intent == intent && read.ReservationID == original.ReservationID && (read.State == orgresource.ReservationCommitted || read.State == orgresource.ReservationReleased) {
			return read, nil
		}
		return orgresource.ConsumerChargeReceipt{}, err
	}
	return receipt, nil
}

// Advance the durable check time before any external proof read. Permanently
// UNKNOWN reservations cannot occupy every slot in subsequent recovery passes.
func (r *GormConsumerChargeRepository) ClaimDue(ctx context.Context) ([]orgresource.ConsumerChargeIdentity, error) {
	var identities []orgresource.ConsumerChargeIdentity
	err := r.runner.run(ctx, func(tx *gorm.DB) error {
		identities = nil
		now := r.now().UTC()
		next := now.Add(30 * time.Second)
		var rows []organizationResourceReservationRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("charge_protocol = ? AND state = ? AND next_check_at <= ?", consumerChargeProtocol, orgresource.ReservationReserved, now).Order("next_check_at, created_at, reservation_id").Limit(25).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			updated := tx.Model(&organizationResourceReservationRow{}).Where("organization_id = ? AND reservation_id = ? AND state = ?", row.OrganizationID, row.ReservationID, orgresource.ReservationReserved).Update("next_check_at", next)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return orgresource.ErrConcurrencyRetry
			}
			identities = append(identities, consumerChargeReceipt(row).Intent.Identity)
		}
		return nil
	})
	return identities, err
}
func readConsumerReservation(db *gorm.DB, identity orgresource.ConsumerChargeIdentity, lock bool) (organizationResourceReservationRow, error) {
	var row organizationResourceReservationRow
	query := db.Where("organization_id = ? AND owner_type = ? AND owner_attempt_id = ? AND charge_protocol = ?", identity.OrganizationID, identity.Consumer, identity.OperationID, consumerChargeProtocol)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.Take(&row).Error
	return row, err
}
func consumerChargeReceipt(row organizationResourceReservationRow) orgresource.ConsumerChargeReceipt {
	return orgresource.ConsumerChargeReceipt{Intent: orgresource.ConsumerChargeIntent{Identity: orgresource.ConsumerChargeIdentity{OrganizationID: row.OrganizationID, Consumer: orgresource.ResourceConsumer(row.OwnerType), OperationID: row.OwnerAttemptID}, ActorID: row.ChargeActorID, MemberID: row.MemberID, Funding: orgresource.ResourceFunding(row.ChargeFunding), ResourceType: orgresource.ResourceType(row.ResourceType), Quantity: row.Quantity, Fingerprint: row.RequestFingerprint, BusinessScope: row.BusinessScope}, ReservationID: row.ReservationID, State: orgresource.ReservationState(row.State), OwnerEvidenceID: row.ChargeEvidenceID, CreatedAt: row.CreatedAt}
}
func updateConsumerBucket(tx *gorm.DB, bucket organizationResourceBucketRow, now time.Time) error {
	updated := tx.Model(&organizationResourceBucketRow{}).Where("organization_id = ? AND resource_type = ?", bucket.OrganizationID, bucket.ResourceType).Updates(map[string]any{"available": bucket.Available, "allocated": bucket.Allocated, "reserved": bucket.Reserved, "consumed": bucket.Consumed, "updated_at": now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return orgresource.ErrConcurrencyRetry
	}
	return nil
}
func completeChargeOperation(tx *gorm.DB, op organizationResourceOperationRow, receipt orgresource.ConsumerChargeReceipt, now time.Time) error {
	payload, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	updated := tx.Model(&organizationResourceOperationRow{}).Where("organization_id = ? AND operation_id = ? AND state = ?", op.OrganizationID, op.OperationID, "processing").Updates(map[string]any{"state": "succeeded", "immutable_result_snapshot": string(payload), "approval_evidence_id": receipt.OwnerEvidenceID, "completed_at": now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return orgresource.ErrConcurrencyRetry
	}
	return nil
}
func createConsumerEvent(tx *gorm.DB, row organizationResourceReservationRow, bucket organizationResourceBucketRow, operationID, reason string, availableDelta, allocatedDelta, reservedDelta, consumedDelta int64, credit positiveCreditAllocation, now time.Time) error {
	if err := tx.Create(&organizationResourceEventRow{EventID: uuid.NewString(), OrganizationID: row.OrganizationID, OperationID: operationID, ReservationID: &row.ReservationID, ResourceType: row.ResourceType, Quantity: row.Quantity, AvailableDelta: availableDelta, AllocatedDelta: allocatedDelta, ReservedDelta: reservedDelta, ConsumedDelta: consumedDelta, Reason: reason, SourceType: row.OwnerType, SourceIdentity: row.OwnerAttemptID, BalanceAfter: bucket.Available, AvailableAfter: bucket.Available, AllocatedAfter: bucket.Allocated, ReservedAfter: bucket.Reserved, ConsumedAfter: bucket.Consumed, GrossCredit: credit.gross, DebtRepaid: credit.debtRepaid, NetCredit: credit.net, CreatedAt: now}).Error; err != nil {
		return err
	}
	payload, err := json.Marshal(consumerChargeReceipt(row))
	if err != nil {
		return err
	}
	return tx.Create(&organizationResourceAuditLogRow{OrganizationID: row.OrganizationID, OperationID: operationID, Action: reason, ActorID: row.ChargeActorID, Payload: string(payload), CreatedAt: now}).Error
}
