package orgresourceadapter

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"task-processor/internal/imageagent"
	"task-processor/internal/ledger/orgresource"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This fixed adapter consumes immutable facts from the ImageAgent V3 owner.
// It never accepts a caller-selected quantity, price or settlement decision.
type ImageGenerationFactReader interface {
	ReadGenerationFact(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.GenerationFact, error)
}
type ImageGenerationAuthorizer interface {
	AuthorizeImageGeneration(context.Context, imageagent.GenerationIntent) error
}
type GormImageGenerationRepository struct {
	db         *gorm.DB
	runner     *transactionRunner
	owner      ImageGenerationFactReader
	authorizer ImageGenerationAuthorizer
	now        func() time.Time
}

func NewGormImageGenerationRepository(db *gorm.DB, config TransactionConfig, owner ImageGenerationFactReader, authorizer ImageGenerationAuthorizer) (*GormImageGenerationRepository, error) {
	if db == nil || owner == nil || authorizer == nil {
		return nil, orgresource.ErrInvalidInput
	}
	return &GormImageGenerationRepository{db: db, runner: newTransactionRunner(db, config), owner: owner, authorizer: authorizer, now: time.Now}, nil
}
func (r *GormImageGenerationRepository) ReserveImageGeneration(ctx context.Context, id imageagent.SlotExternalEffectIdentity) (imageagent.GenerationReservationReceipt, error) {
	fact, err := r.readFact(ctx, id)
	if err != nil {
		return imageagent.GenerationReservationReceipt{}, err
	}
	if err = r.authorizer.AuthorizeImageGeneration(ctx, fact.Intent); err != nil {
		return imageagent.GenerationReservationReceipt{}, err
	}
	var receipt imageagent.GenerationReservationReceipt
	err = r.runner.run(ctx, func(tx *gorm.DB) error {
		op, err := lockFixedResourceOperation(tx, id.TenantID, "image-reserve:"+fact.IntentID, "image_generation_reserve", fact.Fingerprint)
		if err != nil {
			return err
		}
		if op.State == "succeeded" {
			return json.Unmarshal([]byte(op.ImmutableResult), &receipt)
		}
		if op.State != "processing" || fact.State != imageagent.GenerationPrepared {
			return orgresource.ErrOwnerNotReservable
		}
		now := r.now().UTC()
		intent := fact.Intent
		if orgresource.AIPointMonthStart(now) != intent.MonthStart {
			return orgresource.ErrMemberLimitVersionConflict
		}
		var config memberAIPointLimitRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ?", id.TenantID, intent.MemberID).Take(&config).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.ErrMemberLimitUnavailable
		} else if err != nil {
			return err
		}
		if config.Version != intent.LimitVersion {
			return orgresource.ErrMemberLimitVersionConflict
		}
		counter, err := lockMemberMonth(tx, id.TenantID, intent.MemberID, intent.MonthStart)
		if err != nil {
			return err
		}
		if config.MonthlyLimit < counter.Consumed || config.MonthlyLimit-counter.Consumed < counter.Reserved || config.MonthlyLimit-counter.Consumed-counter.Reserved < intent.Points {
			return orgresource.ErrMemberLimitExceeded
		}
		var bucket organizationResourceBucketRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", id.TenantID, orgresource.ResourceAIPoint).Take(&bucket).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.ErrInsufficientBalance
		} else if err != nil {
			return err
		}
		if bucket.Available < intent.Points {
			return orgresource.ErrInsufficientBalance
		}
		if bucket.Reserved > math.MaxInt64-intent.Points {
			return orgresource.ErrInvalidInput
		}
		receipt = imageagent.GenerationReservationReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OrganizationID: id.TenantID, MemberID: intent.MemberID, OperationID: op.OperationID, ReservationID: uuid.NewString(), ResourceType: string(orgresource.ResourceAIPoint), Points: intent.Points, PriceVersion: intent.PriceVersion, LimitVersion: intent.LimitVersion, MonthStart: intent.MonthStart}
		reservation := organizationResourceReservationRow{OrganizationID: id.TenantID, ReservationID: receipt.ReservationID, OperationID: op.OperationID, OwnerType: "image_generation_v1", OwnerAttemptID: fact.IntentID, BusinessScope: id.RunID, ResourceType: string(orgresource.ResourceAIPoint), ReservationPurpose: "single_source_edit", Quantity: intent.Points, State: string(orgresource.ReservationReserved), RequestFingerprint: fact.Fingerprint, MemberID: intent.MemberID, MemberMonthStart: &intent.MonthStart, MemberLimitVersion: intent.LimitVersion, PriceVersion: intent.PriceVersion}
		if err := tx.Omit("Events").Create(&reservation).Error; err != nil {
			return err
		}
		counter.Reserved += intent.Points
		if err := tx.Save(&counter).Error; err != nil {
			return err
		}
		bucket.Available -= intent.Points
		bucket.Reserved += intent.Points
		if err := saveImageBucket(tx, bucket, now); err != nil {
			return err
		}
		payload, _ := json.Marshal(receipt)
		if err := completeImageOperation(tx, op, string(payload), now); err != nil {
			return err
		}
		if err := tx.Create(&organizationResourceEventRow{EventID: uuid.NewString(), OrganizationID: id.TenantID, OperationID: op.OperationID, ReservationID: &receipt.ReservationID, ResourceType: string(orgresource.ResourceAIPoint), Quantity: intent.Points, AvailableDelta: -intent.Points, ReservedDelta: intent.Points, Reason: orgresource.OperationReserve, SourceType: "image_generation_v1", SourceIdentity: fact.IntentID, BalanceAfter: bucket.Available, AvailableAfter: bucket.Available, ReservedAfter: bucket.Reserved, ConsumedAfter: bucket.Consumed}).Error; err != nil {
			return err
		}
		return tx.Create(&organizationResourceAuditLogRow{OrganizationID: id.TenantID, OperationID: op.OperationID, Action: "image_generation_reserve", ActorID: id.OwnerUserID, Payload: string(payload)}).Error
	})
	if err != nil {
		return imageagent.GenerationReservationReceipt{}, err
	}
	// Validate persisted replay rather than trusting an arbitrary operation blob.
	if _, err := fact.BindReservation(receipt); err != nil && fact.Reservation != receipt {
		return imageagent.GenerationReservationReceipt{}, orgresource.ErrInvalidOwnerProof
	}
	return receipt, nil
}
func (r *GormImageGenerationRepository) FinalizeImageGeneration(ctx context.Context, id imageagent.SlotExternalEffectIdentity) (imageagent.GenerationSettlementReceipt, error) {
	fact, err := r.readFact(ctx, id)
	if err != nil {
		return imageagent.GenerationSettlementReceipt{}, err
	}
	if fact.State != imageagent.GenerationSucceeded && fact.State != imageagent.GenerationNoEffect {
		return imageagent.GenerationSettlementReceipt{}, orgresource.ErrOwnerNotTerminal
	}
	proof := fact.TerminalProofDigest()
	var result imageagent.GenerationSettlementReceipt
	err = r.runner.run(ctx, func(tx *gorm.DB) error {
		reserveOp, err := lockFixedResourceOperation(tx, id.TenantID, "image-reserve:"+fact.IntentID, "image_generation_reserve", fact.Fingerprint)
		if err != nil {
			return err
		}
		finalOp, err := lockFixedResourceOperation(tx, id.TenantID, "image-finalize:"+fact.IntentID, "image_generation_finalize", proof)
		if err != nil {
			return err
		}
		if finalOp.State == "succeeded" {
			return json.Unmarshal([]byte(finalOp.ImmutableResult), &result)
		}
		if finalOp.State != "processing" {
			return orgresource.ErrIdempotencyKeyConflict
		}
		now := r.now().UTC()
		result = imageagent.GenerationSettlementReceipt{IntentID: fact.IntentID, Fingerprint: fact.Fingerprint, OperationID: finalOp.OperationID, ProofDigest: proof, Points: fact.Intent.Points}
		if reserveOp.State == "processing" {
			if fact.State != imageagent.GenerationNoEffect {
				return orgresource.ErrInvalidOwnerProof
			}
			// Close the original operation even when reserve has not arrived.
			// A late stale prepared snapshot must not create an orphan hold.
			if err := tx.Model(&organizationResourceOperationRow{}).Where("organization_id = ? AND operation_id = ?", id.TenantID, reserveOp.OperationID).Updates(map[string]any{"state": "failed", "failure_code": "image_no_generation", "completed_at": now}).Error; err != nil {
				return err
			}
			result.State = "no_reservation"
		} else {
			if reserveOp.State != "succeeded" {
				return orgresource.ErrIdempotencyKeyConflict
			}
			var receipt imageagent.GenerationReservationReceipt
			if err := json.Unmarshal([]byte(reserveOp.ImmutableResult), &receipt); err != nil {
				return err
			}
			var reservation organizationResourceReservationRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND reservation_id = ?", id.TenantID, receipt.ReservationID).Take(&reservation).Error; err != nil {
				return err
			}
			if !matchesImageReservation(fact, reservation) || reservation.State != string(orgresource.ReservationReserved) {
				return orgresource.ErrInvalidOwnerProof
			}
			if fact.State == imageagent.GenerationSucceeded && fact.Reservation != receipt {
				return orgresource.ErrInvalidOwnerProof
			}
			counter, err := lockMemberMonth(tx, id.TenantID, reservation.MemberID, *reservation.MemberMonthStart)
			if err != nil {
				return err
			}
			if counter.Reserved < reservation.Quantity {
				return orgresource.ErrInvalidOwnerProof
			}
			var bucket organizationResourceBucketRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", id.TenantID, orgresource.ResourceAIPoint).Take(&bucket).Error; err != nil {
				return err
			}
			if bucket.Reserved < reservation.Quantity {
				return orgresource.ErrInvalidOwnerProof
			}
			decision := orgresource.SettlementRelease
			result.State = "released"
			if fact.State == imageagent.GenerationSucceeded {
				decision = orgresource.SettlementCommit
				result.State = "committed"
				if counter.Consumed > math.MaxInt64-reservation.Quantity {
					return orgresource.ErrInvalidInput
				}
				counter.Consumed += reservation.Quantity
			}
			// Reuse the canonical debt-first release/commit arithmetic.
			settlement, err := (&GormReservationRepository{}).calculateSettlement(tx.Statement.Context, tx, reservation, bucket, decision)
			if err != nil {
				return err
			}
			counter.Reserved -= reservation.Quantity
			if err := tx.Save(&counter).Error; err != nil {
				return err
			}
			bucket.Available = settlement.availableAfter
			bucket.Reserved = settlement.reservedAfter
			bucket.Consumed = settlement.consumedAfter
			if err := saveImageBucket(tx, bucket, now); err != nil {
				return err
			}
			if err := tx.Model(&organizationResourceReservationRow{}).Where("organization_id = ? AND reservation_id = ?", id.TenantID, reservation.ReservationID).Updates(map[string]any{"state": result.State, "settlement_operation_id": finalOp.OperationID, "settled_at": now}).Error; err != nil {
				return err
			}
			result.ReservationID = reservation.ReservationID
			consumedDelta := int64(0)
			if decision == orgresource.SettlementCommit {
				consumedDelta = reservation.Quantity
			}
			if err := tx.Create(&organizationResourceEventRow{EventID: uuid.NewString(), OrganizationID: id.TenantID, OperationID: finalOp.OperationID, ReservationID: &result.ReservationID, ResourceType: string(orgresource.ResourceAIPoint), Quantity: reservation.Quantity, AvailableDelta: settlement.netCredit, ReservedDelta: -reservation.Quantity, ConsumedDelta: consumedDelta, Reason: string(decision), SourceType: "image_generation_v1", SourceIdentity: fact.IntentID, BalanceAfter: bucket.Available, AvailableAfter: bucket.Available, ReservedAfter: bucket.Reserved, ConsumedAfter: bucket.Consumed, GrossCredit: settlement.grossCredit, DebtRepaid: settlement.debtRepaid, NetCredit: settlement.netCredit}).Error; err != nil {
				return err
			}
		}
		payload, _ := json.Marshal(result)
		if err := completeImageOperation(tx, finalOp, string(payload), now); err != nil {
			return err
		}
		return tx.Create(&organizationResourceAuditLogRow{OrganizationID: id.TenantID, OperationID: finalOp.OperationID, Action: "image_generation_finalize:" + result.State, ActorID: id.OwnerUserID, ApprovalEvidenceID: proof, Payload: string(payload)}).Error
	})
	if err != nil {
		return imageagent.GenerationSettlementReceipt{}, err
	}
	return result, nil
}

func (r *GormImageGenerationRepository) readFact(ctx context.Context, id imageagent.SlotExternalEffectIdentity) (imageagent.GenerationFact, error) {
	fact, err := r.owner.ReadGenerationFact(ctx, id)
	if err != nil {
		return fact, err
	}
	if fact.Validate() != nil || fact.Intent.Identity != id {
		return imageagent.GenerationFact{}, orgresource.ErrInvalidOwnerProof
	}
	return fact, nil
}
func completeImageOperation(tx *gorm.DB, op organizationResourceOperationRow, payload string, now time.Time) error {
	updated := tx.Model(&organizationResourceOperationRow{}).Where("organization_id = ? AND operation_id = ? AND state = ?", op.OrganizationID, op.OperationID, "processing").Updates(map[string]any{"state": "succeeded", "immutable_result_snapshot": payload, "completed_at": now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return orgresource.ErrIdempotencyKeyConflict
	}
	return nil
}
func saveImageBucket(tx *gorm.DB, bucket organizationResourceBucketRow, now time.Time) error {
	updated := tx.Model(&organizationResourceBucketRow{}).Where("organization_id = ? AND resource_type = ?", bucket.OrganizationID, bucket.ResourceType).Updates(map[string]any{"available": bucket.Available, "reserved": bucket.Reserved, "consumed": bucket.Consumed, "updated_at": now})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return orgresource.ErrInvalidOwnerProof
	}
	return nil
}
func matchesImageReservation(f imageagent.GenerationFact, r organizationResourceReservationRow) bool {
	i := f.Intent
	return r.OwnerType == "image_generation_v1" && r.OwnerAttemptID == f.IntentID && r.OperationID == "image-reserve:"+f.IntentID && r.OrganizationID == i.Identity.TenantID && r.BusinessScope == i.Identity.RunID && r.ResourceType == string(orgresource.ResourceAIPoint) && r.ReservationPurpose == "single_source_edit" && r.Quantity == i.Points && r.RequestFingerprint == f.Fingerprint && r.MemberID == i.MemberID && r.MemberMonthStart != nil && r.MemberMonthStart.Equal(i.MonthStart) && r.MemberLimitVersion == i.LimitVersion && r.PriceVersion == i.PriceVersion
}
