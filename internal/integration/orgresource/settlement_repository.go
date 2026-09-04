package orgresourceadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/ledger/orgresource"
)

func (repository *GormReservationRepository) ReplaySettlement(ctx context.Context, replay orgresource.SettlementReplay) (orgresource.SettlementResult, bool, error) {
	if result, found, err := repository.lookupSettlementOperation(ctx, repository.db, replay.OrganizationID, replay.OperationID, replay.RequestFingerprint); err != nil || found {
		return result, found, err
	}
	var reservation organizationResourceReservationRow
	err := repository.db.WithContext(ctx).Where("organization_id = ? AND reservation_id = ?", replay.OrganizationID, replay.ReservationID).Take(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.SettlementResult{}, false, nil
	}
	if err != nil {
		return orgresource.SettlementResult{}, false, err
	}
	return repository.settlementResultFromReservation(ctx, repository.db, reservation, replay.RequestFingerprint)
}

func (repository *GormReservationRepository) ExecuteSettlement(ctx context.Context, input orgresource.SettlementExecution) (orgresource.SettlementResult, error) {
	replay := orgresource.SettlementReplay{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID,
		ReservationID: input.ReservationID, RequestFingerprint: input.RequestFingerprint,
	}
	if result, found, err := repository.ReplaySettlement(ctx, replay); err != nil || found {
		return result, err
	}
	seed, err := repository.loadReservation(ctx, repository.db, input.OrganizationID, input.ReservationID, false)
	if err != nil {
		return orgresource.SettlementResult{}, err
	}
	ownerStore, registered := repository.owners[seed.OwnerType]
	if !registered {
		return orgresource.SettlementResult{}, orgresource.ErrReservationOwnerNotRegistered
	}
	binding := bindingFromReservation(seed)

	var committed orgresource.SettlementResult
	err = repository.runner.run(ctx, func(tx *gorm.DB) error {
		transactionContext := tx.Statement.Context
		if result, found, lookupErr := repository.lookupSettlementOperation(transactionContext, tx, input.OrganizationID, input.OperationID, input.RequestFingerprint); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}

		proof, proofErr := ownerStore.LockTerminalProof(transactionContext, tx, binding)
		if proofErr != nil {
			return proofErr
		}
		reservation, reservationErr := repository.loadReservation(transactionContext, tx, input.OrganizationID, input.ReservationID, true)
		if reservationErr != nil {
			return reservationErr
		}
		if result, found, replayErr := repository.settlementResultFromReservation(transactionContext, tx, reservation, input.RequestFingerprint); replayErr != nil {
			return replayErr
		} else if found {
			committed = result
			return nil
		}
		if !sameReservationIdentity(seed, reservation) {
			return orgresource.ErrIdempotencyKeyConflict
		}
		decision, validationErr := validateTerminalProof(reservation, proof)
		if validationErr != nil {
			return validationErr
		}

		var bucket organizationResourceBucketRow
		if bucketErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND resource_type = ?", reservation.OrganizationID, reservation.ResourceType).
			Take(&bucket).Error; bucketErr != nil {
			return bucketErr
		}
		if bucket.Reserved < reservation.Quantity {
			return errors.New("reservation quantity exceeds durable reserved balance")
		}
		settlement, calculateErr := repository.calculateSettlement(transactionContext, tx, reservation, bucket, decision)
		if calculateErr != nil {
			return calculateErr
		}
		return repository.persistSettlement(transactionContext, tx, input, reservation, proof, settlement, &committed)
	})
	if err == nil && repository.afterCommit != nil {
		err = repository.afterCommit()
	}
	if err == nil {
		return committed, nil
	}
	if result, found, lookupErr := repository.ReplaySettlement(ctx, replay); lookupErr == nil && found {
		result.Replayed = true
		return result, nil
	}
	return orgresource.SettlementResult{}, err
}

type calculatedSettlement struct {
	decision       orgresource.SettlementDecision
	availableAfter int64
	reservedAfter  int64
	consumedAfter  int64
	grossCredit    int64
	debtRepaid     int64
	netCredit      int64
}

func (repository *GormReservationRepository) calculateSettlement(ctx context.Context, tx *gorm.DB, reservation organizationResourceReservationRow, bucket organizationResourceBucketRow, decision orgresource.SettlementDecision) (calculatedSettlement, error) {
	result := calculatedSettlement{
		decision: decision, availableAfter: bucket.Available,
		reservedAfter: bucket.Reserved - reservation.Quantity, consumedAfter: bucket.Consumed,
	}
	if decision == orgresource.SettlementCommit {
		result.consumedAfter = bucket.Consumed + reservation.Quantity
		if result.consumedAfter < bucket.Consumed {
			return calculatedSettlement{}, fmt.Errorf("%w: consumed balance overflow", orgresource.ErrInvalidInput)
		}
		return result, nil
	}

	credit, err := applyPositiveCredit(ctx, tx, reservation.OrganizationID, reservation.ResourceType, reservation.Quantity, bucket.Available, time.Now().UTC())
	if err != nil {
		return calculatedSettlement{}, err
	}
	result.grossCredit = credit.gross
	result.debtRepaid = credit.debtRepaid
	result.netCredit = credit.net
	result.availableAfter = credit.availableAfter
	return result, nil
}

func (repository *GormReservationRepository) persistSettlement(ctx context.Context, tx *gorm.DB, input orgresource.SettlementExecution, reservation organizationResourceReservationRow, proof OwnerTerminalProof, settlement calculatedSettlement, committed *orgresource.SettlementResult) error {
	now := time.Now().UTC()
	eventID := uuid.NewString()
	snapshot := orgresource.SettlementSnapshot{
		OperationID: input.OperationID, ReservationID: reservation.ReservationID, OrganizationID: reservation.OrganizationID,
		OwnerType: reservation.OwnerType, OwnerAttemptID: reservation.OwnerAttemptID, BusinessScope: reservation.BusinessScope,
		ResourceType: orgresource.ResourceType(reservation.ResourceType), Quantity: strconv.FormatInt(reservation.Quantity, 10),
		ReservationPurpose: reservation.ReservationPurpose, Decision: settlement.decision,
		OwnerTerminalState: proof.State, OwnerEvidenceID: proof.EvidenceID,
		GrossCredit: strconv.FormatInt(settlement.grossCredit, 10), DebtRepaid: strconv.FormatInt(settlement.debtRepaid, 10),
		NetCredit: strconv.FormatInt(settlement.netCredit, 10), AvailableAfter: strconv.FormatInt(settlement.availableAfter, 10),
		ReservedAfter: strconv.FormatInt(settlement.reservedAfter, 10), ConsumedAfter: strconv.FormatInt(settlement.consumedAfter, 10), EventID: eventID,
	}
	encodedSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal settlement snapshot: %w", err)
	}
	operation := organizationResourceOperationRow{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, OperationType: input.OperationType,
		RequestFingerprint: input.RequestFingerprint, State: "succeeded", ImmutableResult: string(encodedSnapshot),
		ApprovalEvidenceID: proof.EvidenceID, CompletedAt: &now,
	}
	if createErr := tx.WithContext(ctx).Create(&operation).Error; createErr != nil {
		return createErr
	}
	state := orgresource.ReservationCommitted
	if settlement.decision == orgresource.SettlementRelease {
		state = orgresource.ReservationReleased
	}
	updatedReservation := tx.WithContext(ctx).Model(&organizationResourceReservationRow{}).
		Where("organization_id = ? AND reservation_id = ? AND state IN ?", reservation.OrganizationID, reservation.ReservationID,
			[]string{string(orgresource.ReservationReserved), string(orgresource.ReservationReconciliationRequired)}).
		Updates(map[string]any{"state": state, "settlement_operation_id": input.OperationID, "settled_at": now})
	if updatedReservation.Error != nil {
		return updatedReservation.Error
	}
	if updatedReservation.RowsAffected != 1 {
		return orgresource.ErrIdempotencyKeyConflict
	}
	updatedBucket := tx.WithContext(ctx).Model(&organizationResourceBucketRow{}).
		Where("organization_id = ? AND resource_type = ? AND reserved >= ?", reservation.OrganizationID, reservation.ResourceType, reservation.Quantity).
		Updates(map[string]any{
			"available": settlement.availableAfter, "reserved": settlement.reservedAfter,
			"consumed": settlement.consumedAfter, "updated_at": now,
		})
	if updatedBucket.Error != nil {
		return updatedBucket.Error
	}
	if updatedBucket.RowsAffected != 1 {
		return errors.New("reservation settlement lost the durable bucket fence")
	}
	reservationID := reservation.ReservationID
	availableDelta := int64(0)
	reservedDelta := -reservation.Quantity
	consumedDelta := int64(0)
	if settlement.decision == orgresource.SettlementCommit {
		consumedDelta = reservation.Quantity
	} else {
		availableDelta = settlement.netCredit
	}
	event := organizationResourceEventRow{
		EventID: eventID, OrganizationID: reservation.OrganizationID, OperationID: input.OperationID, ReservationID: &reservationID,
		ResourceType: reservation.ResourceType, Quantity: reservation.Quantity, AvailableDelta: availableDelta,
		ReservedDelta: reservedDelta, ConsumedDelta: consumedDelta, Reason: string(settlement.decision),
		SourceType: reservation.OwnerType, SourceIdentity: reservation.OwnerAttemptID, BalanceAfter: settlement.availableAfter,
		AvailableAfter: settlement.availableAfter, ReservedAfter: settlement.reservedAfter, ConsumedAfter: settlement.consumedAfter,
		GrossCredit: settlement.grossCredit, DebtRepaid: settlement.debtRepaid, NetCredit: settlement.netCredit,
	}
	if createErr := tx.WithContext(ctx).Omit("Operation").Create(&event).Error; createErr != nil {
		return createErr
	}
	audit := organizationResourceAuditLogRow{
		OrganizationID: reservation.OrganizationID, OperationID: input.OperationID,
		Action: input.OperationType + ":" + string(settlement.decision), ActorID: input.ActorID,
		ApprovalEvidenceID: proof.EvidenceID, Payload: string(encodedSnapshot),
	}
	if createErr := tx.WithContext(ctx).Omit("Operation").Create(&audit).Error; createErr != nil {
		return createErr
	}
	*committed = orgresource.SettlementResult{Snapshot: snapshot}
	return nil
}

func (repository *GormReservationRepository) loadReservation(ctx context.Context, db *gorm.DB, organizationID, reservationID string, lock bool) (organizationResourceReservationRow, error) {
	query := db.WithContext(ctx)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var reservation organizationResourceReservationRow
	err := query.Where("organization_id = ? AND reservation_id = ?", organizationID, reservationID).Take(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return organizationResourceReservationRow{}, orgresource.ErrReservationNotFound
	}
	return reservation, err
}

func (repository *GormReservationRepository) lookupSettlementOperation(ctx context.Context, db *gorm.DB, organizationID, operationID, fingerprint string) (orgresource.SettlementResult, bool, error) {
	var row organizationResourceOperationRow
	err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", organizationID, operationID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.SettlementResult{}, false, nil
	}
	if err != nil {
		return orgresource.SettlementResult{}, false, err
	}
	if row.OperationType != orgresource.OperationSettleReservation || row.RequestFingerprint != fingerprint {
		return orgresource.SettlementResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	return settlementResultFromOperation(row)
}

func (repository *GormReservationRepository) settlementResultFromReservation(ctx context.Context, db *gorm.DB, reservation organizationResourceReservationRow, fingerprint string) (orgresource.SettlementResult, bool, error) {
	if reservation.SettlementOperationID == nil {
		return orgresource.SettlementResult{}, false, nil
	}
	var operation organizationResourceOperationRow
	if err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", reservation.OrganizationID, *reservation.SettlementOperationID).Take(&operation).Error; err != nil {
		return orgresource.SettlementResult{}, false, err
	}
	if operation.RequestFingerprint != fingerprint || operation.OperationType != orgresource.OperationSettleReservation {
		return orgresource.SettlementResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	result, found, err := settlementResultFromOperation(operation)
	if err == nil {
		result.Replayed = true
	}
	return result, found, err
}

func settlementResultFromOperation(row organizationResourceOperationRow) (orgresource.SettlementResult, bool, error) {
	if row.State != "succeeded" || row.ImmutableResult == "" {
		return orgresource.SettlementResult{}, false, errors.New("organization resource settlement operation is not terminal-successful")
	}
	var snapshot orgresource.SettlementSnapshot
	if err := json.Unmarshal([]byte(row.ImmutableResult), &snapshot); err != nil {
		return orgresource.SettlementResult{}, false, fmt.Errorf("decode immutable settlement result: %w", err)
	}
	return orgresource.SettlementResult{Snapshot: snapshot, Replayed: true}, true, nil
}

func bindingFromReservation(reservation organizationResourceReservationRow) OwnerReservationBinding {
	return OwnerReservationBinding{
		OrganizationID: reservation.OrganizationID, OwnerType: reservation.OwnerType, OwnerAttemptID: reservation.OwnerAttemptID,
		ReservationID: reservation.ReservationID, ResourceType: orgresource.ResourceType(reservation.ResourceType),
		ReservationPurpose: reservation.ReservationPurpose,
	}
}

func validateTerminalProof(reservation organizationResourceReservationRow, proof OwnerTerminalProof) (orgresource.SettlementDecision, error) {
	if proof.OrganizationID != reservation.OrganizationID || proof.OwnerType != reservation.OwnerType ||
		proof.OwnerAttemptID != reservation.OwnerAttemptID || proof.BusinessScope != reservation.BusinessScope ||
		proof.ReservationID != reservation.ReservationID || proof.ResourceType != orgresource.ResourceType(reservation.ResourceType) ||
		proof.ReservationPurpose != reservation.ReservationPurpose {
		return "", orgresource.ErrOwnerScopeMismatch
	}
	var decision orgresource.SettlementDecision
	switch proof.State {
	case orgresource.OwnerAttemptSucceededTerminal:
		decision = orgresource.SettlementCommit
	case orgresource.OwnerAttemptFailedTerminal, orgresource.OwnerAttemptCancelledTerminal:
		decision = orgresource.SettlementRelease
	default:
		return "", orgresource.ErrOwnerNotTerminal
	}
	if strings.TrimSpace(proof.EvidenceID) == "" || len(strings.TrimSpace(proof.EvidenceID)) > 192 {
		return "", orgresource.ErrInvalidOwnerProof
	}
	return decision, nil
}

func sameReservationIdentity(left, right organizationResourceReservationRow) bool {
	return left.OrganizationID == right.OrganizationID && left.ReservationID == right.ReservationID &&
		left.OperationID == right.OperationID && left.OwnerType == right.OwnerType && left.OwnerAttemptID == right.OwnerAttemptID &&
		left.BusinessScope == right.BusinessScope && left.ResourceType == right.ResourceType &&
		left.ReservationPurpose == right.ReservationPurpose && left.Quantity == right.Quantity &&
		left.RequestFingerprint == right.RequestFingerprint
}
