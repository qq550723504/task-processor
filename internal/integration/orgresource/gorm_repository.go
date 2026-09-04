package orgresourceadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"task-processor/internal/ledger/orgresource"
)

type GormRepository struct {
	db          *gorm.DB
	runner      *transactionRunner
	afterCommit func() error
}

func NewGormRepository(db *gorm.DB, config TransactionConfig) (*GormRepository, error) {
	if db == nil {
		return nil, errors.New("organization resource database is required")
	}
	return &GormRepository{db: db, runner: newTransactionRunner(db, config)}, nil
}

func (repository *GormRepository) ReplayWelcomeGrant(ctx context.Context, replay orgresource.WelcomeGrantReplay) (orgresource.WelcomeGrantResult, bool, error) {
	if result, found, err := repository.lookupOperation(ctx, repository.db, replay.OrganizationID, replay.OperationID, replay.RequestFingerprint); err != nil || found {
		return result, found, err
	}
	return repository.lookupSource(ctx, repository.db, orgresource.WelcomeGrantExecution{
		OrganizationID:     replay.OrganizationID,
		OperationID:        replay.OperationID,
		ResourceType:       replay.ResourceType,
		SourceType:         replay.SourceType,
		SourceIdentity:     replay.SourceIdentity,
		RequestFingerprint: replay.RequestFingerprint,
	})
}

func (repository *GormRepository) ExecuteWelcomeGrant(ctx context.Context, input orgresource.WelcomeGrantExecution) (orgresource.WelcomeGrantResult, error) {
	if result, found, err := repository.lookupOperation(ctx, repository.db, input.OrganizationID, input.OperationID, input.RequestFingerprint); err != nil || found {
		return result, err
	}
	if result, found, err := repository.lookupSource(ctx, repository.db, input); err != nil || found {
		return result, err
	}

	var committed orgresource.WelcomeGrantResult
	err := repository.runner.run(ctx, func(tx *gorm.DB) error {
		transactionContext := tx.Statement.Context
		if result, found, lookupErr := repository.lookupOperation(transactionContext, tx, input.OrganizationID, input.OperationID, input.RequestFingerprint); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}
		if result, found, lookupErr := repository.lookupSource(transactionContext, tx, input); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}

		bucket := organizationResourceBucketRow{OrganizationID: input.OrganizationID, ResourceType: string(input.ResourceType)}
		if createErr := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&bucket).Error; createErr != nil {
			return createErr
		}
		if lockErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("organization_id = ? AND resource_type = ?", input.OrganizationID, input.ResourceType).
			Take(&bucket).Error; lockErr != nil {
			return lockErr
		}
		// The bucket lock serializes source re-check with an earlier committer on
		// READ COMMITTED databases. SERIALIZABLE PostgreSQL may instead abort and
		// retry this transaction, which reaches the same read-back path.
		if result, found, lookupErr := repository.lookupSource(transactionContext, tx, input); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}

		now := time.Now().UTC()
		credit, creditErr := applyPositiveCredit(transactionContext, tx, input.OrganizationID, string(input.ResourceType), input.Quantity, bucket.Available, now)
		if creditErr != nil {
			return creditErr
		}
		balanceAfter := credit.availableAfter
		eventID := uuid.NewString()
		snapshot := orgresource.WelcomeGrantSnapshot{
			OperationID:    input.OperationID,
			OrganizationID: input.OrganizationID,
			ResourceType:   input.ResourceType,
			Quantity:       strconv.FormatInt(input.Quantity, 10),
			BalanceAfter:   strconv.FormatInt(balanceAfter, 10),
			SourceType:     input.SourceType,
			SourceIdentity: input.SourceIdentity,
			EventID:        eventID,
		}
		encodedSnapshot, marshalErr := json.Marshal(snapshot)
		if marshalErr != nil {
			return fmt.Errorf("marshal welcome grant snapshot: %w", marshalErr)
		}
		operation := organizationResourceOperationRow{
			OrganizationID:     input.OrganizationID,
			OperationID:        input.OperationID,
			OperationType:      input.OperationType,
			RequestFingerprint: input.RequestFingerprint,
			State:              "succeeded",
			ImmutableResult:    string(encodedSnapshot),
			ApprovalEvidenceID: input.ApprovalEvidenceID,
			CompletedAt:        &now,
		}
		if createErr := tx.Create(&operation).Error; createErr != nil {
			return createErr
		}
		claim := organizationResourceSourceClaimRow{
			SourceType:         input.SourceType,
			SourceIdentity:     input.SourceIdentity,
			ResourceType:       string(input.ResourceType),
			OrganizationID:     input.OrganizationID,
			OperationID:        input.OperationID,
			RequestFingerprint: input.RequestFingerprint,
		}
		if createErr := tx.Omit("Operation").Create(&claim).Error; createErr != nil {
			return createErr
		}
		if updateErr := tx.Model(&organizationResourceBucketRow{}).
			Where("organization_id = ? AND resource_type = ?", input.OrganizationID, input.ResourceType).
			Updates(map[string]any{"available": balanceAfter, "updated_at": now}).Error; updateErr != nil {
			return updateErr
		}
		event := organizationResourceEventRow{
			EventID:        eventID,
			OrganizationID: input.OrganizationID,
			OperationID:    input.OperationID,
			ResourceType:   string(input.ResourceType),
			Quantity:       input.Quantity,
			AvailableDelta: credit.net,
			Reason:         input.SourceType,
			SourceType:     input.SourceType,
			SourceIdentity: input.SourceIdentity,
			BalanceAfter:   balanceAfter,
			AvailableAfter: balanceAfter,
			ReservedAfter:  bucket.Reserved,
			ConsumedAfter:  bucket.Consumed,
			GrossCredit:    credit.gross,
			DebtRepaid:     credit.debtRepaid,
			NetCredit:      credit.net,
		}
		if createErr := tx.Omit("Operation").Create(&event).Error; createErr != nil {
			return createErr
		}
		auditPayload, marshalErr := json.Marshal(struct {
			ResourceType   orgresource.ResourceType `json:"resource_type"`
			Quantity       string                   `json:"quantity"`
			BalanceAfter   string                   `json:"balance_after"`
			SourceType     string                   `json:"source_type"`
			SourceIdentity string                   `json:"source_identity"`
		}{input.ResourceType, strconv.FormatInt(input.Quantity, 10), strconv.FormatInt(balanceAfter, 10), input.SourceType, input.SourceIdentity})
		if marshalErr != nil {
			return fmt.Errorf("marshal welcome grant audit: %w", marshalErr)
		}
		audit := organizationResourceAuditLogRow{
			OrganizationID:     input.OrganizationID,
			OperationID:        input.OperationID,
			Action:             input.OperationType,
			ActorID:            input.ActorID,
			ApprovalEvidenceID: input.ApprovalEvidenceID,
			Payload:            string(auditPayload),
		}
		if createErr := tx.Omit("Operation").Create(&audit).Error; createErr != nil {
			return createErr
		}
		committed = orgresource.WelcomeGrantResult{Snapshot: snapshot}
		return nil
	})
	if err == nil && repository.afterCommit != nil {
		err = repository.afterCommit()
	}
	if err == nil {
		return committed, nil
	}
	// A COMMIT acknowledgement can be lost after PostgreSQL made every row
	// durable. Always read the authoritative operation and source claim before
	// deciding whether a failed call is safe to retry.
	if result, found, lookupErr := repository.lookupOperation(ctx, repository.db, input.OrganizationID, input.OperationID, input.RequestFingerprint); lookupErr == nil && found {
		result.Replayed = true
		return result, nil
	}
	if result, found, lookupErr := repository.lookupSource(ctx, repository.db, input); lookupErr == nil && found {
		result.Replayed = true
		return result, nil
	}
	return orgresource.WelcomeGrantResult{}, err
}

func (repository *GormRepository) lookupOperation(ctx context.Context, db *gorm.DB, organizationID, operationID, fingerprint string) (orgresource.WelcomeGrantResult, bool, error) {
	var row organizationResourceOperationRow
	err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", organizationID, operationID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.WelcomeGrantResult{}, false, nil
	}
	if err != nil {
		return orgresource.WelcomeGrantResult{}, false, err
	}
	if row.RequestFingerprint != fingerprint {
		return orgresource.WelcomeGrantResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	result, err := resultFromOperation(row)
	return result, err == nil, err
}

func (repository *GormRepository) lookupSource(ctx context.Context, db *gorm.DB, input orgresource.WelcomeGrantExecution) (orgresource.WelcomeGrantResult, bool, error) {
	var claim organizationResourceSourceClaimRow
	err := db.WithContext(ctx).
		Where("source_type = ? AND source_identity = ? AND resource_type = ?", input.SourceType, input.SourceIdentity, input.ResourceType).
		Take(&claim).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return orgresource.WelcomeGrantResult{}, false, nil
	}
	if err != nil {
		return orgresource.WelcomeGrantResult{}, false, err
	}
	if claim.OrganizationID != input.OrganizationID || claim.RequestFingerprint != input.RequestFingerprint {
		return orgresource.WelcomeGrantResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	result, found, err := repository.lookupOperation(ctx, db, claim.OrganizationID, claim.OperationID, claim.RequestFingerprint)
	if err != nil {
		return orgresource.WelcomeGrantResult{}, false, err
	}
	if !found {
		return orgresource.WelcomeGrantResult{}, false, errors.New("organization resource source claim has no terminal operation")
	}
	result.Replayed = true
	return result, true, nil
}

func resultFromOperation(row organizationResourceOperationRow) (orgresource.WelcomeGrantResult, error) {
	if row.State != "succeeded" || row.ImmutableResult == "" {
		return orgresource.WelcomeGrantResult{}, errors.New("organization resource operation is not terminal-successful")
	}
	var snapshot orgresource.WelcomeGrantSnapshot
	if err := json.Unmarshal([]byte(row.ImmutableResult), &snapshot); err != nil {
		return orgresource.WelcomeGrantResult{}, fmt.Errorf("decode immutable resource result: %w", err)
	}
	return orgresource.WelcomeGrantResult{Snapshot: snapshot, Replayed: true}, nil
}
