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

// TrustedCommercialGrantAuthorizer is only used by the server-side commercial
// composition. The browser never supplies this principal or reaches this port.
type TrustedCommercialGrantAuthorizer struct{}

func (TrustedCommercialGrantAuthorizer) AuthorizePurchasedGrant(_ context.Context, principal orgresource.Principal, resourceType orgresource.ResourceType) (orgresource.PurchasedGrantAuthorization, error) {
	if principal.Kind != orgresource.PrincipalTrustedCommercial || strings.TrimSpace(principal.ID) != "commercial-billing" || !validPurchasedResourceType(resourceType) {
		return orgresource.PurchasedGrantAuthorization{}, orgresource.ErrForbidden
	}
	return orgresource.PurchasedGrantAuthorization{MaxQuantity: 1_000_000_000}, nil
}

func validPurchasedResourceType(resourceType orgresource.ResourceType) bool {
	switch resourceType {
	case orgresource.ResourceStoreRenewalPeriod, orgresource.ResourceAIPoint, orgresource.ResourceDataRow:
		return true
	default:
		return false
	}
}

func (repository *GormRepository) ReplayPurchasedResourceGrant(ctx context.Context, replay orgresource.PurchasedResourceGrantReplay) (orgresource.PurchasedResourceGrantResult, bool, error) {
	var result orgresource.PurchasedResourceGrantResult
	var found bool
	err := repository.runner.runRead(ctx, func(readContext context.Context) error {
		var readErr error
		result, found, readErr = repository.replayPurchasedGrantOnce(readContext, repository.db, replay)
		return readErr
	})
	return result, found, err
}

func (repository *GormRepository) replayPurchasedGrantOnce(ctx context.Context, db *gorm.DB, replay orgresource.PurchasedResourceGrantReplay) (orgresource.PurchasedResourceGrantResult, bool, error) {
	var operation organizationResourceOperationRow
	err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", replay.OrganizationID, replay.OperationID).Take(&operation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		var claim organizationResourceSourceClaimRow
		claimErr := db.WithContext(ctx).Where("source_type = ? AND source_identity = ? AND resource_type = ?", replay.SourceType, replay.SourceIdentity, replay.ResourceType).Take(&claim).Error
		if errors.Is(claimErr, gorm.ErrRecordNotFound) {
			return orgresource.PurchasedResourceGrantResult{}, false, nil
		}
		if claimErr != nil {
			return orgresource.PurchasedResourceGrantResult{}, false, claimErr
		}
		if claim.OrganizationID != replay.OrganizationID || claim.RequestFingerprint != replay.RequestFingerprint {
			return orgresource.PurchasedResourceGrantResult{}, false, orgresource.ErrIdempotencyKeyConflict
		}
		return repository.purchasedGrantResultFromOperation(ctx, db, claim.OrganizationID, claim.OperationID, replay.RequestFingerprint)
	}
	if err != nil {
		return orgresource.PurchasedResourceGrantResult{}, false, err
	}
	if operation.RequestFingerprint != replay.RequestFingerprint || operation.OperationType != orgresource.OperationGrantCommercialPurchase {
		return orgresource.PurchasedResourceGrantResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	result, found, resultErr := repository.purchasedGrantResultFromOperation(ctx, db, operation.OrganizationID, operation.OperationID, replay.RequestFingerprint)
	return result, found, resultErr
}

func (repository *GormRepository) purchasedGrantResultFromOperation(ctx context.Context, db *gorm.DB, organizationID, operationID, fingerprint string) (orgresource.PurchasedResourceGrantResult, bool, error) {
	var operation organizationResourceOperationRow
	if err := db.WithContext(ctx).Where("organization_id = ? AND operation_id = ?", organizationID, operationID).Take(&operation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return orgresource.PurchasedResourceGrantResult{}, false, nil
		}
		return orgresource.PurchasedResourceGrantResult{}, false, err
	}
	if operation.RequestFingerprint != fingerprint {
		return orgresource.PurchasedResourceGrantResult{}, false, orgresource.ErrIdempotencyKeyConflict
	}
	if operation.State != "succeeded" || operation.ImmutableResult == "" {
		return orgresource.PurchasedResourceGrantResult{}, false, orgresource.ErrConcurrencyRetry
	}
	var snapshot orgresource.PurchasedResourceGrantSnapshot
	if err := json.Unmarshal([]byte(operation.ImmutableResult), &snapshot); err != nil {
		return orgresource.PurchasedResourceGrantResult{}, false, fmt.Errorf("decode purchased grant snapshot: %w", err)
	}
	return orgresource.PurchasedResourceGrantResult{Snapshot: snapshot, Replayed: true}, true, nil
}

func (repository *GormRepository) ExecutePurchasedResourceGrant(ctx context.Context, input orgresource.PurchasedResourceGrantExecution) (orgresource.PurchasedResourceGrantResult, error) {
	replay := orgresource.PurchasedResourceGrantReplay{OrganizationID: input.OrganizationID, OperationID: input.OperationID, CommercialOrderID: input.CommercialOrderID, CommercialOrderItemID: input.CommercialOrderItemID, ResourceType: input.ResourceType, Quantity: input.Quantity, SourceType: input.SourceType, SourceIdentity: input.SourceIdentity, RequestFingerprint: input.RequestFingerprint}
	if result, found, err := repository.ReplayPurchasedResourceGrant(ctx, replay); err != nil || found {
		return result, err
	}
	var committed orgresource.PurchasedResourceGrantResult
	err := repository.runner.run(ctx, func(tx *gorm.DB) error {
		transactionContext := tx.Statement.Context
		if result, found, lookupErr := repository.replayPurchasedGrantOnce(transactionContext, tx, replay); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}
		var bucket organizationResourceBucketRow
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&organizationResourceBucketRow{OrganizationID: input.OrganizationID, ResourceType: string(input.ResourceType)}).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND resource_type = ?", input.OrganizationID, input.ResourceType).Take(&bucket).Error; err != nil {
			return err
		}
		if result, found, lookupErr := repository.replayPurchasedGrantOnce(transactionContext, tx, replay); lookupErr != nil {
			return lookupErr
		} else if found {
			committed = result
			return nil
		}
		credit, err := applyPositiveCredit(transactionContext, tx, input.OrganizationID, string(input.ResourceType), input.Quantity, bucket.Available, time.Now().UTC())
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		eventID := uuid.NewString()
		snapshot := orgresource.PurchasedResourceGrantSnapshot{OperationID: input.OperationID, OrganizationID: input.OrganizationID, CommercialOrderID: input.CommercialOrderID, CommercialOrderItemID: input.CommercialOrderItemID, ResourceType: input.ResourceType, Quantity: strconv.FormatInt(input.Quantity, 10), BalanceAfter: strconv.FormatInt(credit.availableAfter, 10), SourceType: input.SourceType, SourceIdentity: input.SourceIdentity, EventID: eventID}
		encoded, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		if err := tx.Create(&organizationResourceOperationRow{OrganizationID: input.OrganizationID, OperationID: input.OperationID, OperationType: input.OperationType, RequestFingerprint: input.RequestFingerprint, State: "succeeded", ImmutableResult: string(encoded), CompletedAt: &now}).Error; err != nil {
			return err
		}
		if err := tx.Create(&organizationResourceSourceClaimRow{SourceType: input.SourceType, SourceIdentity: input.SourceIdentity, ResourceType: string(input.ResourceType), OrganizationID: input.OrganizationID, OperationID: input.OperationID, RequestFingerprint: input.RequestFingerprint}).Error; err != nil {
			return err
		}
		if err := tx.Model(&organizationResourceBucketRow{}).Where("organization_id = ? AND resource_type = ?", input.OrganizationID, input.ResourceType).Updates(map[string]any{"available": credit.availableAfter, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Create(&organizationResourceEventRow{EventID: eventID, OrganizationID: input.OrganizationID, OperationID: input.OperationID, ResourceType: string(input.ResourceType), Quantity: input.Quantity, AvailableDelta: credit.net, Reason: input.OperationType, SourceType: input.SourceType, SourceIdentity: input.SourceIdentity, BalanceAfter: credit.availableAfter, AvailableAfter: credit.availableAfter, ReservedAfter: bucket.Reserved, ConsumedAfter: bucket.Consumed, GrossCredit: credit.gross, DebtRepaid: credit.debtRepaid, NetCredit: credit.net, CreatedAt: now}).Error; err != nil {
			return err
		}
		payload, _ := json.Marshal(snapshot)
		if err := tx.Create(&organizationResourceAuditLogRow{OrganizationID: input.OrganizationID, OperationID: input.OperationID, Action: input.OperationType, ActorID: input.ActorID, Payload: string(payload), CreatedAt: now}).Error; err != nil {
			return err
		}
		committed = orgresource.PurchasedResourceGrantResult{Snapshot: snapshot}
		return nil
	})
	if err == nil {
		return committed, nil
	}
	if result, found, lookupErr := repository.ReplayPurchasedResourceGrant(ctx, replay); lookupErr == nil && found {
		result.Replayed = true
		return result, nil
	}
	return orgresource.PurchasedResourceGrantResult{}, err
}
