package orgresourceadapter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
)

// TransactionalStoreServiceStore is the Store half of the shared
// Store+Resource unit of work. Implementations must use the supplied tx for
// both the row lock and the versioned write.
type TransactionalStoreServiceStore interface {
	LockServiceState(context.Context, *gorm.DB, storecenter.ServiceStoreIdentity) (storecenter.ServiceStoreSnapshot, error)
	ApplyServiceState(context.Context, *gorm.DB, storecenter.ServiceStoreMutation) error
}

type StoreServiceExecutor struct {
	db          *gorm.DB
	runner      *transactionRunner
	stores      TransactionalStoreServiceStore
	afterCommit func() error
}

type storeServiceFailureSnapshot struct {
	OrganizationID       string                     `json:"organization_id"`
	OperationID          string                     `json:"operation_id"`
	StoreID              string                     `json:"store_id"`
	Command              storecenter.ServiceCommand `json:"command"`
	Quantity             string                     `json:"quantity"`
	ExpectedStoreVersion int64                      `json:"expected_store_version"`
	FailureCode          string                     `json:"failure_code"`
}

func NewStoreServiceExecutor(db *gorm.DB, config TransactionConfig, stores TransactionalStoreServiceStore) (*StoreServiceExecutor, error) {
	if db == nil {
		return nil, errors.New("organization resource database is required")
	}
	if stores == nil {
		return nil, errors.New("transactional Store service adapter is required")
	}
	return &StoreServiceExecutor{db: db, runner: newTransactionRunner(db, config), stores: stores}, nil
}

var _ storecenter.ServiceLifecycleExecutor = (*StoreServiceExecutor)(nil)
var _ TransactionalStoreServiceStore = (*storecenter.GormStoreRepository)(nil)

func (executor *StoreServiceExecutor) ReplayServiceLifecycle(ctx context.Context, replay storecenter.ServiceReplay) (storecenter.ServiceOperationResult, bool, error) {
	var result storecenter.ServiceOperationResult
	var found bool
	err := executor.runner.runRead(ctx, func(readContext context.Context) error {
		var readErr error
		result, found, readErr = executor.lookupOperation(readContext, executor.db, replay.OrganizationID, replay.OperationID, replay.RequestFingerprint, false)
		return readErr
	})
	return result, found, err
}

func (executor *StoreServiceExecutor) ExecuteServiceLifecycle(ctx context.Context, input storecenter.ServiceExecution) (storecenter.ServiceOperationResult, error) {
	if err := validateServiceExecution(input); err != nil {
		return storecenter.ServiceOperationResult{}, err
	}
	if result, found, err := executor.ReplayServiceLifecycle(ctx, storecenter.ServiceReplay{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, RequestFingerprint: input.RequestFingerprint,
	}); err != nil || found {
		return result, err
	}

	var committed storecenter.ServiceOperationResult
	var terminalFailure error
	err := executor.runner.run(ctx, func(tx *gorm.DB) error {
		transactionContext := tx.Statement.Context
		if result, found, lookupErr := executor.lookupOperation(transactionContext, tx, input.OrganizationID, input.OperationID, input.RequestFingerprint, true); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}

		locked, lockErr := executor.stores.LockServiceState(transactionContext, tx, storecenter.ServiceStoreIdentity{
			OrganizationID: input.OrganizationID,
			StoreID:        input.StoreID,
		})
		if lockErr != nil {
			return lockErr
		}
		if locked.Identity.OrganizationID != input.OrganizationID || locked.Identity.StoreID != input.StoreID {
			return storecenter.ErrNotFound
		}
		if locked.Version != input.ExpectedStoreVersion {
			terminalFailure = storecenter.ErrVersionConflict
			return executor.persistTerminalFailure(transactionContext, tx, input, terminalFailure)
		}
		if locked.ConnectionRef != input.ExpectedConnectionRef {
			return storecenter.ErrConnectionSnapshotChanged
		}
		target, transitionErr := targetServiceState(locked.State, input)
		if transitionErr != nil {
			if _, _, terminal := terminalServiceFailure(transitionErr); terminal {
				terminalFailure = transitionErr
				return executor.persistTerminalFailure(transactionContext, tx, input, terminalFailure)
			}
			return transitionErr
		}

		var bucket organizationResourceBucketRow
		bucketErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND resource_type = ?", input.OrganizationID, orgresource.ResourceStoreRenewalPeriod).
			Take(&bucket).Error
		if errors.Is(bucketErr, gorm.ErrRecordNotFound) {
			terminalFailure = orgresource.ErrInsufficientBalance
			return executor.persistTerminalFailure(transactionContext, tx, input, terminalFailure)
		}
		if bucketErr != nil {
			return bucketErr
		}
		if bucket.Available < input.Quantity {
			terminalFailure = orgresource.ErrInsufficientBalance
			return executor.persistTerminalFailure(transactionContext, tx, input, terminalFailure)
		}
		if input.Quantity > math.MaxInt64-bucket.Consumed {
			return fmt.Errorf("%w: consumed balance overflow", orgresource.ErrInvalidInput)
		}

		now := input.OccurredAt.UTC()
		availableAfter := bucket.Available - input.Quantity
		consumedAfter := bucket.Consumed + input.Quantity
		eventID := uuid.NewString()
		snapshot := storecenter.ServiceOperationSnapshot{
			OrganizationID: input.OrganizationID,
			OperationID:    input.OperationID,
			StoreID:        input.StoreID,
			Command:        input.Command,
			Quantity:       strconv.FormatInt(input.Quantity, 10),
			ResourceType:   string(orgresource.ResourceStoreRenewalPeriod),
			BalanceAfter:   strconv.FormatInt(availableAfter, 10),
			StoreVersion:   locked.Version + 1,
			ServiceState:   target,
			EventID:        eventID,
		}
		encodedSnapshot, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return fmt.Errorf("marshal Store service operation snapshot: %w", marshalErr)
		}
		operation := organizationResourceOperationRow{
			OrganizationID: input.OrganizationID, OperationID: input.OperationID, OperationType: string(input.Command),
			RequestFingerprint: input.RequestFingerprint, State: "succeeded", ImmutableResult: string(encodedSnapshot), CompletedAt: &now,
		}
		if createErr := tx.Create(&operation).Error; createErr != nil {
			return createErr
		}
		bucketUpdate := tx.Model(&organizationResourceBucketRow{}).
			Where("organization_id = ? AND resource_type = ? AND available >= ?", input.OrganizationID, orgresource.ResourceStoreRenewalPeriod, input.Quantity).
			Updates(map[string]any{"available": availableAfter, "consumed": consumedAfter, "updated_at": now})
		if bucketUpdate.Error != nil {
			return bucketUpdate.Error
		}
		if bucketUpdate.RowsAffected != 1 {
			return orgresource.ErrInsufficientBalance
		}
		if applyErr := executor.stores.ApplyServiceState(transactionContext, tx, storecenter.ServiceStoreMutation{
			Identity:              locked.Identity,
			ExpectedVersion:       locked.Version,
			ExpectedConnectionRef: locked.ConnectionRef,
			State:                 target,
			ActorSubject:          input.ActorSubject,
			OccurredAt:            now,
		}); applyErr != nil {
			return applyErr
		}
		event := organizationResourceEventRow{
			EventID: eventID, OrganizationID: input.OrganizationID, OperationID: input.OperationID,
			ResourceType: string(orgresource.ResourceStoreRenewalPeriod), Quantity: input.Quantity,
			AvailableDelta: -input.Quantity, ConsumedDelta: input.Quantity, Reason: string(input.Command),
			SourceType: "store_service", SourceIdentity: input.StoreID, BalanceAfter: availableAfter,
			AvailableAfter: availableAfter, ReservedAfter: bucket.Reserved, ConsumedAfter: consumedAfter,
		}
		if createErr := tx.Omit("Operation").Create(&event).Error; createErr != nil {
			return createErr
		}
		auditPayload, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return fmt.Errorf("marshal Store service audit: %w", marshalErr)
		}
		audit := organizationResourceAuditLogRow{
			OrganizationID: input.OrganizationID, OperationID: input.OperationID, Action: string(input.Command),
			ActorID: input.ActorSubject, Payload: string(auditPayload),
		}
		if createErr := tx.Omit("Operation").Create(&audit).Error; createErr != nil {
			return createErr
		}
		committed = storecenter.ServiceOperationResult{Snapshot: snapshot}
		return nil
	})
	if err == nil && executor.afterCommit != nil {
		err = executor.afterCommit()
	}
	if err == nil {
		if terminalFailure != nil {
			return storecenter.ServiceOperationResult{}, terminalFailure
		}
		return committed, nil
	}
	if result, found, lookupErr := executor.ReplayServiceLifecycle(ctx, storecenter.ServiceReplay{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, RequestFingerprint: input.RequestFingerprint,
	}); found {
		if lookupErr == nil {
			result.Replayed = true
		}
		return result, lookupErr
	}
	return storecenter.ServiceOperationResult{}, err
}

func (executor *StoreServiceExecutor) lookupOperation(ctx context.Context, db *gorm.DB, organizationID, operationID, fingerprint string, lock bool) (storecenter.ServiceOperationResult, bool, error) {
	query := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", organizationID, operationID)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row organizationResourceOperationRow
	err := query.Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return storecenter.ServiceOperationResult{}, false, nil
	}
	if err != nil {
		return storecenter.ServiceOperationResult{}, false, err
	}
	if row.RequestFingerprint != fingerprint {
		return storecenter.ServiceOperationResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	if row.ImmutableResult == "" || !validServiceCommand(storecenter.ServiceCommand(row.OperationType)) {
		return storecenter.ServiceOperationResult{}, false, errors.New("Store service operation is not terminal-successful")
	}
	if row.State == "failed" {
		var snapshot storeServiceFailureSnapshot
		if err := json.Unmarshal([]byte(row.ImmutableResult), &snapshot); err != nil {
			return storecenter.ServiceOperationResult{}, false, fmt.Errorf("decode Store service failure snapshot: %w", err)
		}
		if snapshot.OrganizationID != organizationID || snapshot.OperationID != operationID || snapshot.StoreID == "" || string(snapshot.Command) != row.OperationType || snapshot.FailureCode != row.FailureCode {
			return storecenter.ServiceOperationResult{}, false, errors.New("Store service failure snapshot identity mismatch")
		}
		failure := serviceFailureFromCode(snapshot.FailureCode)
		if failure == nil {
			return storecenter.ServiceOperationResult{}, false, errors.New("Store service failure code is invalid")
		}
		return storecenter.ServiceOperationResult{}, true, failure
	}
	if row.State != "succeeded" {
		return storecenter.ServiceOperationResult{}, false, errors.New("Store service operation is not terminal")
	}
	var snapshot storecenter.ServiceOperationSnapshot
	if err := json.Unmarshal([]byte(row.ImmutableResult), &snapshot); err != nil {
		return storecenter.ServiceOperationResult{}, false, fmt.Errorf("decode Store service operation snapshot: %w", err)
	}
	if snapshot.OrganizationID != organizationID || snapshot.OperationID != operationID || string(snapshot.Command) != row.OperationType || snapshot.StoreID == "" || snapshot.ResourceType != string(orgresource.ResourceStoreRenewalPeriod) {
		return storecenter.ServiceOperationResult{}, false, errors.New("Store service operation snapshot identity mismatch")
	}
	if err := storecenter.ValidateStoreServiceState(snapshot.ServiceState); err != nil {
		return storecenter.ServiceOperationResult{}, false, err
	}
	return storecenter.ServiceOperationResult{Snapshot: snapshot, Replayed: true}, true, nil
}

func (executor *StoreServiceExecutor) persistTerminalFailure(ctx context.Context, tx *gorm.DB, input storecenter.ServiceExecution, failure error) error {
	code, httpStatus, terminal := terminalServiceFailure(failure)
	if !terminal {
		return failure
	}
	now := input.OccurredAt.UTC()
	snapshot := storeServiceFailureSnapshot{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, StoreID: input.StoreID,
		Command: input.Command, Quantity: strconv.FormatInt(input.Quantity, 10),
		ExpectedStoreVersion: input.ExpectedStoreVersion, FailureCode: code,
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal Store service terminal failure: %w", err)
	}
	operation := organizationResourceOperationRow{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, OperationType: string(input.Command),
		RequestFingerprint: input.RequestFingerprint, State: "failed", FailureCode: code,
		FailureHTTPStatus: &httpStatus, ImmutableResult: string(encoded), CompletedAt: &now,
	}
	if err := tx.WithContext(ctx).Create(&operation).Error; err != nil {
		return err
	}
	audit := organizationResourceAuditLogRow{
		OrganizationID: input.OrganizationID, OperationID: input.OperationID, Action: string(input.Command),
		ActorID: input.ActorSubject, Payload: string(encoded),
	}
	return tx.WithContext(ctx).Omit("Operation").Create(&audit).Error
}

func terminalServiceFailure(err error) (string, int, bool) {
	switch {
	case errors.Is(err, orgresource.ErrInsufficientBalance):
		return "insufficient_balance", 409, true
	case errors.Is(err, storecenter.ErrVersionConflict):
		return "store_version_conflict", 409, true
	case errors.Is(err, storecenter.ErrInvalidServiceTransition):
		return "invalid_service_transition", 409, true
	case errors.Is(err, storecenter.ErrServiceAlreadyActive):
		return "service_already_active", 409, true
	case errors.Is(err, storecenter.ErrServiceExpired):
		return "service_expired", 409, true
	case errors.Is(err, storecenter.ErrServiceNotExpired):
		return "service_not_expired", 409, true
	case errors.Is(err, storecenter.ErrServiceSuspended):
		return "service_suspended", 409, true
	default:
		return "", 0, false
	}
}

func serviceFailureFromCode(code string) error {
	switch code {
	case "insufficient_balance":
		return orgresource.ErrInsufficientBalance
	case "store_version_conflict":
		return storecenter.ErrVersionConflict
	case "invalid_service_transition":
		return storecenter.ErrInvalidServiceTransition
	case "service_already_active":
		return storecenter.ErrServiceAlreadyActive
	case "service_expired":
		return storecenter.ErrServiceExpired
	case "service_not_expired":
		return storecenter.ErrServiceNotExpired
	case "service_suspended":
		return storecenter.ErrServiceSuspended
	default:
		return nil
	}
}

func validateServiceExecution(input storecenter.ServiceExecution) error {
	if input.OrganizationID == "" || input.OperationID == "" || input.StoreID == "" || input.ActorSubject == "" || input.ExpectedStoreVersion <= 0 || input.OccurredAt.IsZero() {
		return orgresource.ErrInvalidInput
	}
	decoded, err := hex.DecodeString(input.RequestFingerprint)
	if err != nil || len(decoded) != 32 {
		return orgresource.ErrInvalidInput
	}
	if !validServiceCommand(input.Command) {
		return orgresource.ErrInvalidInput
	}
	if input.Command == storecenter.ServiceCommandActivate && input.Quantity != 1 {
		return orgresource.ErrInvalidInput
	}
	if input.Quantity <= 0 || input.MaxQuantity <= 0 || input.Quantity > input.MaxQuantity {
		return orgresource.ErrInvalidInput
	}
	return nil
}

func validServiceCommand(command storecenter.ServiceCommand) bool {
	switch command {
	case storecenter.ServiceCommandActivate, storecenter.ServiceCommandRenew, storecenter.ServiceCommandReactivate:
		return true
	default:
		return false
	}
}

func targetServiceState(current storecenter.StoreServiceState, input storecenter.ServiceExecution) (storecenter.StoreServiceState, error) {
	switch input.Command {
	case storecenter.ServiceCommandActivate:
		return storecenter.ActivateStoreService(current, input.ConnectionStatus, input.OccurredAt)
	case storecenter.ServiceCommandRenew:
		return storecenter.RenewStoreService(current, input.Quantity, input.MaxQuantity, input.OccurredAt)
	case storecenter.ServiceCommandReactivate:
		return storecenter.ReactivateStoreService(current, input.Quantity, input.MaxQuantity, input.OccurredAt)
	default:
		return storecenter.StoreServiceState{}, orgresource.ErrInvalidInput
	}
}
