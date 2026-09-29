package orgresourceadapter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"task-processor/internal/aicapability"
	"task-processor/internal/ledger/orgresource"
	"time"
)

const modelPointOwner = "model_invocation_v1"

type ModelInvocationFactReader interface {
	ReadModelInvocation(context.Context, string, string) (aicapability.InvocationRecord, error)
}
type ModelInvocationAuthorizer interface {
	AuthorizeModelInvocation(context.Context, aicapability.InvocationRecord) error
}

// This adapter is fixed to native Product Agent invocation facts. Token arguments
// only cross-check that fact; they are never an independent charge authority.
type GormModelInvocationRepository struct {
	db         *gorm.DB
	runner     *transactionRunner
	owner      ModelInvocationFactReader
	authorizer ModelInvocationAuthorizer
	now        func() time.Time
}
type modelPointReceipt struct {
	ReservationID, MemberID, Fingerprint, PriceVersion string
	Quantity, LimitVersion                             int64
	Month                                              time.Time
}

func NewGormModelInvocationRepository(db *gorm.DB, config TransactionConfig, owner ModelInvocationFactReader, auth ModelInvocationAuthorizer) (*GormModelInvocationRepository, error) {
	if db == nil || owner == nil || auth == nil {
		return nil, orgresource.ErrInvalidInput
	}
	return &GormModelInvocationRepository{db: db, runner: newTransactionRunner(db, config), owner: owner, authorizer: auth, now: time.Now}, nil
}

func modelPointHash(value any) string {
	raw, _ := json.Marshal(value)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
func modelPointKey(kind, id string) string { return "model-point-" + kind + ":" + modelPointHash(id) }
func modelPointFingerprint(fact aicapability.InvocationRecord) string {
	fact.StartedAt = fact.StartedAt.UTC().Truncate(time.Microsecond)
	fact.FinishedAt = time.Time{}
	fact.LatencyMilliseconds = 0
	fact.PromptTokens, fact.CompletionTokens, fact.TotalTokens, fact.ImageCount = 0, 0, 0, 0
	fact.EstimatedCostMicros, fact.EstimatedCostKnown, fact.UsageKnown = 0, false, false
	fact.Outcome = aicapability.InvocationDispatched
	fact.ErrorCategory, fact.ErrorCode, fact.RouteErrorCategory = "", "", ""
	fact.ProviderRequestID, fact.UpstreamJobID, fact.OutputHash = "", "", ""
	fact.ReviewScore, fact.ReviewNeedsHumanReview, fact.ReviewReasons = 0, false, nil
	return modelPointHash(fact)
}
func (r *GormModelInvocationRepository) readFact(ctx context.Context, org, id string) (aicapability.InvocationRecord, error) {
	fact, err := r.owner.ReadModelInvocation(ctx, org, id)
	if err != nil {
		return fact, err
	}
	if fact.TenantID != org || fact.InvocationID != id || fact.Operation != aicapability.OperationProductAgentDecision || fact.MemberID == "" || fact.UserID == "" || fact.InputHash == "" || fact.StartedAt.IsZero() || !fact.PointTariff.Valid() || fact.MaximumPromptTokens <= 0 || fact.MaximumCompletionTokens <= 0 || fact.MaximumPromptTokens > math.MaxInt64-fact.MaximumCompletionTokens {
		return fact, orgresource.ErrInvalidOwnerProof
	}
	if _, err := fact.PointTariff.Points(fact.MaximumPromptTokens, fact.MaximumCompletionTokens); err != nil {
		return fact, orgresource.ErrInvalidOwnerProof
	}
	return fact, nil
}
func (r *GormModelInvocationRepository) ReserveAIInvocationUsage(ctx context.Context, org, member, id string, maximumTokens int64, at time.Time) error {
	fact, err := r.readFact(ctx, org, id)
	if err != nil {
		return err
	}
	if fact.MemberID != member || maximumTokens != fact.MaximumPromptTokens+fact.MaximumCompletionTokens || !at.UTC().Truncate(time.Microsecond).Equal(fact.StartedAt.UTC().Truncate(time.Microsecond)) {
		return orgresource.ErrInvalidOwnerProof
	}
	if err := r.authorizer.AuthorizeModelInvocation(ctx, fact); err != nil {
		return err
	}
	fingerprint := modelPointFingerprint(fact)
	quantity, _ := fact.PointTariff.Points(fact.MaximumPromptTokens, fact.MaximumCompletionTokens)
	return r.runner.run(ctx, func(tx *gorm.DB) error {
		op, err := lockFixedResourceOperation(tx, org, modelPointKey("reserve", id), "model_point_reserve", fingerprint)
		if err != nil {
			return err
		}
		if op.State == "succeeded" {
			var receipt modelPointReceipt
			if json.Unmarshal([]byte(op.ImmutableResult), &receipt) != nil {
				return orgresource.ErrInvalidOwnerProof
			}
			var row organizationResourceReservationRow
			if err := tx.Where("organization_id = ? AND reservation_id = ?", org, receipt.ReservationID).Take(&row).Error; err != nil {
				return err
			}
			if !matchesModelPointReceipt(fact, fingerprint, quantity, receipt, row) {
				return orgresource.ErrInvalidOwnerProof
			}
			return nil
		}
		if op.State != "processing" || fact.Outcome != aicapability.InvocationDispatched {
			return orgresource.ErrOwnerNotReservable
		}
		now := r.now().UTC()
		month := orgresource.AIPointMonthStart(fact.StartedAt)
		if !month.Equal(orgresource.AIPointMonthStart(now)) {
			return orgresource.ErrMemberLimitVersionConflict
		}
		var limit memberAIPointLimitRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND member_id = ?", org, member).Take(&limit).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.ErrMemberLimitUnavailable
		} else if err != nil {
			return err
		}
		counter, err := lockMemberMonth(tx, org, member, month)
		if err != nil {
			return err
		}
		if limit.MonthlyLimit < counter.Consumed || limit.MonthlyLimit-counter.Consumed < counter.Reserved || limit.MonthlyLimit-counter.Consumed-counter.Reserved < quantity {
			return orgresource.ErrMemberLimitExceeded
		}
		var bucket organizationResourceBucketRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", org, orgresource.ResourceAIPoint).Take(&bucket).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.ErrInsufficientBalance
		} else if err != nil {
			return err
		}
		if bucket.Available < quantity {
			return orgresource.ErrInsufficientBalance
		}
		if bucket.Reserved > math.MaxInt64-quantity {
			return orgresource.ErrInvalidInput
		}
		receipt := modelPointReceipt{ReservationID: uuid.NewString(), MemberID: member, Fingerprint: fingerprint, PriceVersion: fact.PointTariff.PriceVersion, Quantity: quantity, LimitVersion: limit.Version, Month: month}
		row := organizationResourceReservationRow{OrganizationID: org, ReservationID: receipt.ReservationID, OperationID: op.OperationID, OwnerType: modelPointOwner, OwnerAttemptID: id, BusinessScope: fact.AgentRunID, ResourceType: string(orgresource.ResourceAIPoint), ReservationPurpose: modelPointOwner, Quantity: quantity, State: string(orgresource.ReservationReserved), RequestFingerprint: fingerprint, MemberID: member, MemberMonthStart: &month, MemberLimitVersion: limit.Version, PriceVersion: receipt.PriceVersion, NextCheckAt: &now, ChargeActorID: fact.UserID, CreatedAt: now}
		if err := tx.Omit("Events").Create(&row).Error; err != nil {
			return err
		}
		counter.Reserved += quantity
		if err := tx.Save(&counter).Error; err != nil {
			return err
		}
		bucket.Available -= quantity
		bucket.Reserved += quantity
		if err := updateConsumerBucket(tx, bucket, now); err != nil {
			return err
		}
		payload, _ := json.Marshal(receipt)
		if err := completeImageOperation(tx, op, string(payload), now); err != nil {
			return err
		}
		return writeModelPointEvent(tx, row, bucket, op.OperationID, "model_point_reserve", -quantity, quantity, 0, positiveCreditAllocation{}, string(payload), now)
	})
}

func matchesModelPointReceipt(fact aicapability.InvocationRecord, fingerprint string, quantity int64, receipt modelPointReceipt, row organizationResourceReservationRow) bool {
	month := orgresource.AIPointMonthStart(fact.StartedAt)
	return receipt.ReservationID != "" && receipt.MemberID == fact.MemberID && receipt.Fingerprint == fingerprint && receipt.PriceVersion == fact.PointTariff.PriceVersion && receipt.Quantity == quantity && receipt.LimitVersion > 0 && receipt.Month.Equal(month) && row.OrganizationID == fact.TenantID && row.OwnerType == modelPointOwner && row.OwnerAttemptID == fact.InvocationID && row.BusinessScope == fact.AgentRunID && row.ResourceType == string(orgresource.ResourceAIPoint) && row.ReservationPurpose == modelPointOwner && row.Quantity == quantity && row.RequestFingerprint == fingerprint && row.MemberID == receipt.MemberID && row.MemberMonthStart != nil && row.MemberMonthStart.Equal(month) && row.MemberLimitVersion == receipt.LimitVersion && row.PriceVersion == receipt.PriceVersion && row.ChargeActorID == fact.UserID
}

func modelPointTerminal(fact aicapability.InvocationRecord) (int64, bool, error) {
	if fact.FinishedAt.IsZero() || fact.FinishedAt.Before(fact.StartedAt) {
		return 0, false, orgresource.ErrOwnerNotTerminal
	}
	if fact.Outcome == aicapability.InvocationFailed && fact.UsageKnown && fact.PromptTokens == 0 && fact.CompletionTokens == 0 && fact.TotalTokens == 0 && (fact.ErrorCode == "reservation_failed_before_dispatch" || fact.ErrorCode == "rejected_before_dispatch") {
		return 0, true, nil
	}
	if (fact.Outcome != aicapability.InvocationSucceeded && fact.Outcome != aicapability.InvocationUsageObservedFailed) || !fact.UsageKnown {
		return 0, false, orgresource.ErrOwnerNotTerminal
	}
	if fact.PromptTokens < 0 || fact.CompletionTokens < 0 || fact.TotalTokens <= 0 || int64(fact.PromptTokens) > fact.MaximumPromptTokens || int64(fact.CompletionTokens) > fact.MaximumCompletionTokens || int64(fact.TotalTokens) != int64(fact.PromptTokens)+int64(fact.CompletionTokens) {
		return 0, false, orgresource.ErrInvalidOwnerProof
	}
	points, err := fact.PointTariff.Points(int64(fact.PromptTokens), int64(fact.CompletionTokens))
	return points, false, err
}
func (r *GormModelInvocationRepository) SettleAIInvocationUsage(ctx context.Context, org, member, id string, total int64, at time.Time) error {
	fact, err := r.readFact(ctx, org, id)
	if err != nil {
		return err
	}
	if fact.MemberID != member || total != int64(fact.TotalTokens) || !at.UTC().Truncate(time.Microsecond).Equal(fact.FinishedAt.UTC().Truncate(time.Microsecond)) {
		return orgresource.ErrInvalidOwnerProof
	}
	return r.finalize(ctx, fact)
}
func (r *GormModelInvocationRepository) ReleaseAIInvocationUsage(ctx context.Context, org, id string) error {
	fact, err := r.readFact(ctx, org, id)
	if err != nil {
		return err
	}
	_, noDispatch, err := modelPointTerminal(fact)
	if err != nil {
		return err
	}
	if !noDispatch {
		return orgresource.ErrInvalidOwnerProof
	}
	return r.finalize(ctx, fact)
}
func (r *GormModelInvocationRepository) finalize(ctx context.Context, fact aicapability.InvocationRecord) error {
	actual, noDispatch, err := modelPointTerminal(fact)
	if err != nil {
		return err
	}
	quantity, _ := fact.PointTariff.Points(fact.MaximumPromptTokens, fact.MaximumCompletionTokens)
	fingerprint := modelPointFingerprint(fact)
	fact.StartedAt = fact.StartedAt.UTC().Truncate(time.Microsecond)
	fact.FinishedAt = fact.FinishedAt.UTC().Truncate(time.Microsecond)
	proof := modelPointHash(fact)
	org, id := fact.TenantID, fact.InvocationID
	return r.runner.run(ctx, func(tx *gorm.DB) error {
		reserve, err := lockFixedResourceOperation(tx, org, modelPointKey("reserve", id), "model_point_reserve", fingerprint)
		if err != nil {
			return err
		}
		final, err := lockFixedResourceOperation(tx, org, modelPointKey("finalize", id), "model_point_finalize", proof)
		if err != nil {
			return err
		}
		if final.State == "succeeded" {
			return nil
		}
		if final.State != "processing" {
			return orgresource.ErrIdempotencyKeyConflict
		}
		now := r.now().UTC()
		state := "released"
		if !noDispatch {
			state = "committed"
		}
		result := struct {
			State, Proof string
			Points       int64
		}{state, proof, actual}
		payload, _ := json.Marshal(result)
		if reserve.State == "processing" {
			if !noDispatch {
				return orgresource.ErrInvalidOwnerProof
			}
			if err := tx.Model(&organizationResourceOperationRow{}).Where("organization_id = ? AND operation_id = ? AND state = ?", org, reserve.OperationID, "processing").Updates(map[string]any{"state": "failed", "failure_code": "model_not_dispatched", "completed_at": now}).Error; err != nil {
				return err
			}
		} else {
			if reserve.State != "succeeded" {
				return orgresource.ErrIdempotencyKeyConflict
			}
			var receipt modelPointReceipt
			if json.Unmarshal([]byte(reserve.ImmutableResult), &receipt) != nil {
				return orgresource.ErrInvalidOwnerProof
			}
			var row organizationResourceReservationRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND reservation_id = ?", org, receipt.ReservationID).Take(&row).Error; err != nil {
				return err
			}
			if !matchesModelPointReceipt(fact, fingerprint, quantity, receipt, row) || (row.State != string(orgresource.ReservationReserved) && row.State != string(orgresource.ReservationReconciliationRequired)) || actual > quantity {
				return orgresource.ErrInvalidOwnerProof
			}
			counter, err := lockMemberMonth(tx, org, row.MemberID, receipt.Month)
			if err != nil {
				return err
			}
			var bucket organizationResourceBucketRow
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", org, orgresource.ResourceAIPoint).Take(&bucket).Error; err != nil {
				return err
			}
			if counter.Reserved < quantity || bucket.Reserved < quantity || counter.Consumed > math.MaxInt64-actual || bucket.Consumed > math.MaxInt64-actual {
				return orgresource.ErrInvalidOwnerProof
			}
			credit := positiveCreditAllocation{availableAfter: bucket.Available}
			if quantity > actual {
				credit, err = applyPositiveCredit(ctx, tx, org, string(orgresource.ResourceAIPoint), quantity-actual, bucket.Available, now)
				if err != nil {
					return err
				}
			}
			counter.Reserved -= quantity
			counter.Consumed += actual
			if err := tx.Save(&counter).Error; err != nil {
				return err
			}
			bucket.Reserved -= quantity
			bucket.Consumed += actual
			bucket.Available = credit.availableAfter
			if err := updateConsumerBucket(tx, bucket, now); err != nil {
				return err
			}
			if err := tx.Model(&organizationResourceReservationRow{}).Where("organization_id = ? AND reservation_id = ?", org, row.ReservationID).Updates(map[string]any{"state": state, "settlement_operation_id": final.OperationID, "settled_at": now, "next_check_at": nil, "charge_evidence_id": proof}).Error; err != nil {
				return err
			}
			if err := writeModelPointEvent(tx, row, bucket, final.OperationID, "model_point_"+state, credit.net, -quantity, actual, credit, string(payload), now); err != nil {
				return err
			}
		}
		return completeImageOperation(tx, final, string(payload), now)
	})
}
func writeModelPointEvent(tx *gorm.DB, row organizationResourceReservationRow, bucket organizationResourceBucketRow, op, reason string, available, reserved, consumed int64, credit positiveCreditAllocation, payload string, now time.Time) error {
	if err := tx.Create(&organizationResourceEventRow{EventID: uuid.NewString(), OrganizationID: row.OrganizationID, OperationID: op, ReservationID: &row.ReservationID, ResourceType: row.ResourceType, Quantity: row.Quantity, AvailableDelta: available, ReservedDelta: reserved, ConsumedDelta: consumed, Reason: reason, SourceType: modelPointOwner, SourceIdentity: row.OwnerAttemptID, BalanceAfter: bucket.Available, AvailableAfter: bucket.Available, AllocatedAfter: bucket.Allocated, ReservedAfter: bucket.Reserved, ConsumedAfter: bucket.Consumed, GrossCredit: credit.gross, DebtRepaid: credit.debtRepaid, NetCredit: credit.net, CreatedAt: now}).Error; err != nil {
		return err
	}
	return tx.Create(&organizationResourceAuditLogRow{OrganizationID: row.OrganizationID, OperationID: op, Action: reason, ActorID: row.ChargeActorID, Payload: payload, CreatedAt: now}).Error
}

// Only original model reservations are scanned; neither new reservations nor
// provider calls are made. Advance check time before reading any owner proof.
func (r *GormModelInvocationRepository) RecoverDue(ctx context.Context) (int, error) {
	var rows []organizationResourceReservationRow
	err := r.runner.run(ctx, func(tx *gorm.DB) error {
		rows = nil
		now := r.now().UTC()
		next := now.Add(30 * time.Second)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("owner_type = ? AND reservation_purpose = ? AND state IN ? AND next_check_at <= ?", modelPointOwner, modelPointOwner, []string{string(orgresource.ReservationReserved), string(orgresource.ReservationReconciliationRequired)}, now).Order("next_check_at, created_at, reservation_id").Limit(50).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			if err := tx.Model(&organizationResourceReservationRow{}).Where("organization_id = ? AND reservation_id = ?", row.OrganizationID, row.ReservationID).Update("next_check_at", next).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	completed := 0
	var failures []error
	for _, row := range rows {
		fact, err := r.readFact(ctx, row.OrganizationID, row.OwnerAttemptID)
		if err == nil {
			err = r.finalize(ctx, fact)
		}
		if err == nil {
			completed++
		} else if !errors.Is(err, orgresource.ErrOwnerNotTerminal) {
			failures = append(failures, err)
		}
	}
	return completed, errors.Join(failures...)
}

var _ aicapability.InvocationUsageSettler = (*GormModelInvocationRepository)(nil)
var _ aicapability.InvocationUsageReservation = (*GormModelInvocationRepository)(nil)
