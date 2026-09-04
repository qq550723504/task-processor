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

// TransactionalReservationOwnerStore is implemented by each approved owner
// domain. Both methods receive the resource transaction, so locking the exact
// owner attempt and writing its reservation binding share the commit boundary
// with the resource operation. Implementations must issue a row lock and must
// fail unless exactly one canonical owner attempt is updated.
type TransactionalReservationOwnerStore interface {
	LockOwnerAttempt(ctx context.Context, tx *gorm.DB, identity OwnerAttemptIdentity) (OwnerAttemptSnapshot, error)
	BindReservation(ctx context.Context, tx *gorm.DB, binding OwnerReservationBinding) error
	LockTerminalProof(ctx context.Context, tx *gorm.DB, binding OwnerReservationBinding) (OwnerTerminalProof, error)
}

type OwnerAttemptIdentity struct {
	OrganizationID string
	OwnerType      string
	OwnerAttemptID string
}

type OwnerAttemptSnapshot struct {
	OrganizationID     string
	OwnerType          string
	OwnerAttemptID     string
	BusinessScope      string
	State              orgresource.OwnerAttemptState
	ReservationID      string
	ResourceType       orgresource.ResourceType
	ReservationPurpose string
}

type OwnerReservationBinding struct {
	OrganizationID     string
	OwnerType          string
	OwnerAttemptID     string
	ReservationID      string
	ResourceType       orgresource.ResourceType
	ReservationPurpose string
}

type OwnerTerminalProof struct {
	OrganizationID     string
	OwnerType          string
	OwnerAttemptID     string
	BusinessScope      string
	ReservationID      string
	ResourceType       orgresource.ResourceType
	ReservationPurpose string
	State              orgresource.OwnerAttemptState
	EvidenceID         string
}

type GormReservationRepository struct {
	db          *gorm.DB
	runner      *transactionRunner
	owners      map[string]TransactionalReservationOwnerStore
	afterCommit func() error
}

func NewGormReservationRepository(db *gorm.DB, config TransactionConfig, owners map[string]TransactionalReservationOwnerStore) (*GormReservationRepository, error) {
	if db == nil {
		return nil, errors.New("organization resource database is required")
	}
	registered := make(map[string]TransactionalReservationOwnerStore, len(owners))
	for ownerType, owner := range owners {
		canonicalType := strings.TrimSpace(ownerType)
		if canonicalType == "" || owner == nil {
			return nil, errors.New("reservation owner registration is invalid")
		}
		registered[canonicalType] = owner
	}
	return &GormReservationRepository{db: db, runner: newTransactionRunner(db, config), owners: registered}, nil
}

func (repository *GormReservationRepository) ReplayReservation(ctx context.Context, replay orgresource.ReservationReplay) (orgresource.ReservationResult, bool, error) {
	var result orgresource.ReservationResult
	var found bool
	err := repository.runner.runRead(ctx, func(readContext context.Context) error {
		var readErr error
		result, found, readErr = repository.replayReservationOnce(readContext, replay)
		return readErr
	})
	return result, found, err
}

func (repository *GormReservationRepository) replayReservationOnce(ctx context.Context, replay orgresource.ReservationReplay) (orgresource.ReservationResult, bool, error) {
	if result, found, err := repository.lookupOperation(ctx, repository.db, replay.OrganizationID, replay.OperationID, replay.RequestFingerprint); err != nil || found {
		return result, found, err
	}
	return repository.lookupReservationIdentity(ctx, repository.db, replay)
}

func (repository *GormReservationRepository) ExecuteReservation(ctx context.Context, input orgresource.ReservationExecution) (orgresource.ReservationResult, error) {
	ownerStore, registered := repository.owners[input.OwnerType]
	if !registered {
		return orgresource.ReservationResult{}, orgresource.ErrReservationOwnerNotRegistered
	}
	replay := reservationReplayFromExecution(input)
	if result, found, err := repository.ReplayReservation(ctx, replay); err != nil || found {
		return result, err
	}

	var committed orgresource.ReservationResult
	err := repository.runner.run(ctx, func(tx *gorm.DB) error {
		transactionContext := tx.Statement.Context
		if result, found, lookupErr := repository.lookupOperation(transactionContext, tx, input.OrganizationID, input.OperationID, input.RequestFingerprint); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}
		if result, found, lookupErr := repository.lookupReservationIdentity(transactionContext, tx, replay); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}

		identity := OwnerAttemptIdentity{OrganizationID: input.OrganizationID, OwnerType: input.OwnerType, OwnerAttemptID: input.OwnerAttemptID}
		owner, lockErr := ownerStore.LockOwnerAttempt(transactionContext, tx, identity)
		if errors.Is(lockErr, gorm.ErrRecordNotFound) {
			return orgresource.ErrOwnerScopeMismatch
		}
		if lockErr != nil {
			return lockErr
		}
		if err := validateLockedOwner(input, owner); err != nil {
			return err
		}
		// Re-check after taking the owner fence. A concurrent operation may have
		// committed the exact reservation while this transaction was waiting.
		if result, found, lookupErr := repository.lookupReservationIdentity(transactionContext, tx, replay); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}
		if owner.ReservationID != "" {
			result, lookupErr := repository.lookupBoundReservation(transactionContext, tx, owner, replay)
			if lookupErr != nil {
				return lookupErr
			}
			committed = result
			return nil
		}
		if owner.State != orgresource.OwnerAttemptNotStarted {
			return orgresource.ErrOwnerNotReservable
		}

		var bucket organizationResourceBucketRow
		if bucketErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND resource_type = ?", input.OrganizationID, input.ResourceType).
			Take(&bucket).Error; errors.Is(bucketErr, gorm.ErrRecordNotFound) {
			return orgresource.ErrInsufficientBalance
		} else if bucketErr != nil {
			return bucketErr
		}
		if bucket.Available < input.Quantity {
			return orgresource.ErrInsufficientBalance
		}

		now := time.Now().UTC()
		reservationID := uuid.NewString()
		eventID := uuid.NewString()
		availableAfter := bucket.Available - input.Quantity
		reservedAfter := bucket.Reserved + input.Quantity
		if reservedAfter < bucket.Reserved {
			return fmt.Errorf("%w: reserved balance overflow", orgresource.ErrInvalidInput)
		}
		snapshot := orgresource.ReservationSnapshot{
			OperationID: input.OperationID, ReservationID: reservationID, OrganizationID: input.OrganizationID,
			OwnerType: input.OwnerType, OwnerAttemptID: input.OwnerAttemptID, BusinessScope: input.BusinessScope,
			ResourceType: input.ResourceType, Quantity: strconv.FormatInt(input.Quantity, 10),
			ReservationPurpose: input.ReservationPurpose, State: orgresource.ReservationReserved,
			AvailableAfter: strconv.FormatInt(availableAfter, 10), ReservedAfter: strconv.FormatInt(reservedAfter, 10),
			ConsumedAfter: strconv.FormatInt(bucket.Consumed, 10), EventID: eventID,
		}
		encodedSnapshot, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return fmt.Errorf("marshal reservation snapshot: %w", marshalErr)
		}
		operation := organizationResourceOperationRow{
			OrganizationID: input.OrganizationID, OperationID: input.OperationID, OperationType: input.OperationType,
			RequestFingerprint: input.RequestFingerprint, State: "succeeded", ImmutableResult: string(encodedSnapshot), CompletedAt: &now,
		}
		if createErr := tx.Create(&operation).Error; createErr != nil {
			return createErr
		}
		reservation := organizationResourceReservationRow{
			OrganizationID: input.OrganizationID, ReservationID: reservationID, OperationID: input.OperationID,
			OwnerType: input.OwnerType, OwnerAttemptID: input.OwnerAttemptID, BusinessScope: input.BusinessScope,
			ResourceType: string(input.ResourceType), ReservationPurpose: input.ReservationPurpose,
			Quantity: input.Quantity, State: string(orgresource.ReservationReserved), RequestFingerprint: input.RequestFingerprint,
		}
		if createErr := tx.Omit("Events").Create(&reservation).Error; createErr != nil {
			return createErr
		}
		binding := OwnerReservationBinding{
			OrganizationID: input.OrganizationID, OwnerType: input.OwnerType, OwnerAttemptID: input.OwnerAttemptID,
			ReservationID: reservationID, ResourceType: input.ResourceType, ReservationPurpose: input.ReservationPurpose,
		}
		if bindErr := ownerStore.BindReservation(transactionContext, tx, binding); bindErr != nil {
			return bindErr
		}
		boundOwner, verifyErr := ownerStore.LockOwnerAttempt(transactionContext, tx, identity)
		if verifyErr != nil {
			return verifyErr
		}
		if !ownerMatchesBinding(boundOwner, binding) {
			return errors.New("reservation owner adapter did not persist the exact binding")
		}
		updated := tx.Model(&organizationResourceBucketRow{}).
			Where("organization_id = ? AND resource_type = ? AND available >= ?", input.OrganizationID, input.ResourceType, input.Quantity).
			Updates(map[string]any{"available": availableAfter, "reserved": reservedAfter, "updated_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return orgresource.ErrInsufficientBalance
		}
		event := organizationResourceEventRow{
			EventID: eventID, OrganizationID: input.OrganizationID, OperationID: input.OperationID, ReservationID: &reservationID,
			ResourceType: string(input.ResourceType), Quantity: input.Quantity, AvailableDelta: -input.Quantity,
			ReservedDelta: input.Quantity, Reason: orgresource.OperationReserve, SourceType: input.OwnerType,
			SourceIdentity: input.OwnerAttemptID, BalanceAfter: availableAfter, AvailableAfter: availableAfter,
			ReservedAfter: reservedAfter, ConsumedAfter: bucket.Consumed,
		}
		if createErr := tx.Omit("Operation").Create(&event).Error; createErr != nil {
			return createErr
		}
		auditPayload, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return fmt.Errorf("marshal reservation audit: %w", marshalErr)
		}
		audit := organizationResourceAuditLogRow{
			OrganizationID: input.OrganizationID, OperationID: input.OperationID, Action: orgresource.OperationReserve,
			ActorID: input.ActorID, Payload: string(auditPayload),
		}
		if createErr := tx.Omit("Operation").Create(&audit).Error; createErr != nil {
			return createErr
		}
		committed = orgresource.ReservationResult{Snapshot: snapshot}
		return nil
	})
	if err == nil && repository.afterCommit != nil {
		err = repository.afterCommit()
	}
	if err == nil {
		return committed, nil
	}
	if result, found, lookupErr := repository.ReplayReservation(ctx, replay); lookupErr == nil && found {
		result.Replayed = true
		return result, nil
	}
	return orgresource.ReservationResult{}, err
}

func (repository *GormReservationRepository) lookupOperation(ctx context.Context, db *gorm.DB, organizationID, operationID, fingerprint string) (orgresource.ReservationResult, bool, error) {
	var row organizationResourceOperationRow
	err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", organizationID, operationID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.ReservationResult{}, false, nil
	}
	if err != nil {
		return orgresource.ReservationResult{}, false, err
	}
	if row.RequestFingerprint != fingerprint || row.OperationType != orgresource.OperationReserve {
		return orgresource.ReservationResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	return reservationResultFromOperation(row)
}

func (repository *GormReservationRepository) lookupReservationIdentity(ctx context.Context, db *gorm.DB, replay orgresource.ReservationReplay) (orgresource.ReservationResult, bool, error) {
	var row organizationResourceReservationRow
	err := db.WithContext(ctx).Where(
		"organization_id = ? AND owner_type = ? AND owner_attempt_id = ? AND resource_type = ? AND reservation_purpose = ?",
		replay.OrganizationID, replay.OwnerType, replay.OwnerAttemptID, replay.ResourceType, replay.ReservationPurpose,
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.ReservationResult{}, false, nil
	}
	if err != nil {
		return orgresource.ReservationResult{}, false, err
	}
	if row.RequestFingerprint != replay.RequestFingerprint || row.BusinessScope != replay.BusinessScope || row.Quantity != replay.Quantity {
		return orgresource.ReservationResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	var operation organizationResourceOperationRow
	if err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", row.OrganizationID, row.OperationID).Take(&operation).Error; err != nil {
		return orgresource.ReservationResult{}, false, err
	}
	result, found, err := reservationResultFromOperation(operation)
	if err == nil {
		result.Replayed = true
	}
	return result, found, err
}

func (repository *GormReservationRepository) lookupBoundReservation(ctx context.Context, db *gorm.DB, owner OwnerAttemptSnapshot, replay orgresource.ReservationReplay) (orgresource.ReservationResult, error) {
	var row organizationResourceReservationRow
	if err := db.WithContext(ctx).Where("organization_id = ? AND reservation_id = ?", owner.OrganizationID, owner.ReservationID).Take(&row).Error; err != nil {
		return orgresource.ReservationResult{}, err
	}
	if row.OwnerType != replay.OwnerType || row.OwnerAttemptID != replay.OwnerAttemptID ||
		row.BusinessScope != replay.BusinessScope || row.ResourceType != string(replay.ResourceType) ||
		row.ReservationPurpose != replay.ReservationPurpose || row.Quantity != replay.Quantity ||
		row.RequestFingerprint != replay.RequestFingerprint || owner.ResourceType != replay.ResourceType ||
		owner.ReservationPurpose != replay.ReservationPurpose {
		return orgresource.ReservationResult{}, orgresource.ErrIdempotencyKeyConflict
	}
	var operation organizationResourceOperationRow
	if err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", row.OrganizationID, row.OperationID).Take(&operation).Error; err != nil {
		return orgresource.ReservationResult{}, err
	}
	result, found, err := reservationResultFromOperation(operation)
	if err != nil {
		return orgresource.ReservationResult{}, err
	}
	if !found {
		return orgresource.ReservationResult{}, errors.New("bound reservation has no terminal operation")
	}
	result.Replayed = true
	return result, nil
}

func reservationResultFromOperation(row organizationResourceOperationRow) (orgresource.ReservationResult, bool, error) {
	if row.State != "succeeded" || row.ImmutableResult == "" {
		return orgresource.ReservationResult{}, false, errors.New("organization resource reservation operation is not terminal-successful")
	}
	var snapshot orgresource.ReservationSnapshot
	if err := json.Unmarshal([]byte(row.ImmutableResult), &snapshot); err != nil {
		return orgresource.ReservationResult{}, false, fmt.Errorf("decode immutable reservation result: %w", err)
	}
	return orgresource.ReservationResult{Snapshot: snapshot, Replayed: true}, true, nil
}

func validateLockedOwner(input orgresource.ReservationExecution, owner OwnerAttemptSnapshot) error {
	if owner.OrganizationID != input.OrganizationID || owner.OwnerType != input.OwnerType || owner.OwnerAttemptID != input.OwnerAttemptID || owner.BusinessScope != input.BusinessScope {
		return orgresource.ErrOwnerScopeMismatch
	}
	return nil
}

func ownerMatchesBinding(owner OwnerAttemptSnapshot, binding OwnerReservationBinding) bool {
	return owner.OrganizationID == binding.OrganizationID && owner.OwnerType == binding.OwnerType &&
		owner.OwnerAttemptID == binding.OwnerAttemptID && owner.ReservationID == binding.ReservationID &&
		owner.ResourceType == binding.ResourceType && owner.ReservationPurpose == binding.ReservationPurpose
}

func reservationReplayFromExecution(input orgresource.ReservationExecution) orgresource.ReservationReplay {
	return orgresource.ReservationReplay{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, OwnerType: input.OwnerType,
		OwnerAttemptID: input.OwnerAttemptID, BusinessScope: input.BusinessScope, ResourceType: input.ResourceType,
		Quantity: input.Quantity, ReservationPurpose: input.ReservationPurpose, RequestFingerprint: input.RequestFingerprint,
	}
}
