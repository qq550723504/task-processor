package storecenter

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"task-processor/internal/authidentity"
	"time"
)

type ProductExecutionSubject struct {
	OrganizationID, ActorID, MemberID string
	Purpose                           string
}

const (
	ProductPurposeRules   = "product_rules"
	ProductPurposePublish = "product_publish"
	ProductPurposeImage   = "supply_image_transform"
)

type ProductExecutionAuthorization struct {
	Access  StoreMemberAccess
	Allowed bool
}

// The application verifies current original membership and the exact purpose;
// this port does not manufacture a request identity for a durable command.
type ProductExecutionAuthorizer interface {
	AuthorizeProductExecution(context.Context, ProductExecutionSubject) (ProductExecutionAuthorization, error)
}
type ProductExecutionMaterial struct {
	StoreVersion     int64
	Platform         Platform
	ServiceExpiresAt time.Time
	Connection       OfficialConnectionView
	Attempt          OfficialConnectionAttempt `json:"-"`
}

// ProductMerchantBinding is a public reference to current Store authority. It
// contains no credential and never grants durable execution on its own.
type ProductMerchantBinding struct {
	OrganizationID       string                  `json:"organization_id"`
	StoreID              string                  `json:"store_id"`
	Site                 string                  `json:"site"`
	StoreVersion         int64                   `json:"store_version"`
	ConnectionRevision   int64                   `json:"connection_revision"`
	ApplicationRevision  string                  `json:"application_revision"`
	ApplicationID        string                  `json:"application_id"`
	ApplicationType      OfficialApplicationType `json:"application_type"`
	SupplierIdentityHash string                  `json:"supplier_identity_hash"`
	ServiceExpiresAt     time.Time               `json:"service_expires_at"`
}

func (b ProductMerchantBinding) ValidApplication() bool {
	return b.ApplicationType.Valid() && authidentity.IsBoundedIdentifier(b.ApplicationID) && authidentity.IsBoundedIdentifier(b.ApplicationRevision) && strings.HasSuffix(b.ApplicationRevision, ":"+string(b.ApplicationType))
}

type ProductExecutionReader interface {
	ReadProductExecution(context.Context, ProductExecutionSubject, string, ProductExecutionAuthorizer, time.Time) (ProductExecutionMaterial, error)
}

func (r *MemberScopedStoreRepository) ReadProductExecution(ctx context.Context, subject ProductExecutionSubject, storeID string, authorization ProductExecutionAuthorizer, now time.Time) (ProductExecutionMaterial, error) {
	if ctx != nil && ctx.Err() != nil {
		return ProductExecutionMaterial{}, ErrDependencyUnavailable
	}
	if ctx == nil || r == nil || r.db == nil || isNilDependency(authorization) || now.IsZero() {
		return ProductExecutionMaterial{}, ErrNotFound
	}
	if _, ok := ctx.Deadline(); !ok {
		return ProductExecutionMaterial{}, ErrNotFound
	}
	if _, err := canonicalUUID(storeID); err != nil {
		return ProductExecutionMaterial{}, ErrNotFound
	}
	for _, value := range []string{subject.OrganizationID, subject.ActorID, subject.MemberID} {
		if _, err := validateOpaqueIdentity("execution scope", value, MaxSubjectBytes); err != nil {
			return ProductExecutionMaterial{}, ErrNotFound
		}
	}
	if subject.Purpose != ProductPurposeRules && subject.Purpose != ProductPurposePublish && subject.Purpose != ProductPurposeImage {
		return ProductExecutionMaterial{}, ErrNotFound
	}
	approved, err := authorization.AuthorizeProductExecution(ctx, subject)
	if err != nil {
		return ProductExecutionMaterial{}, productExecutionReadError(err)
	}
	if ctx.Err() != nil {
		return ProductExecutionMaterial{}, ErrDependencyUnavailable
	}
	if !approved.Allowed || approved.Access.OrganizationID != subject.OrganizationID || approved.Access.ActorID != subject.ActorID || approved.Access.MemberID != subject.MemberID {
		return ProductExecutionMaterial{}, ErrNotFound
	}
	var material ProductExecutionMaterial
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockMemberStore(tx, subject.OrganizationID, storeID); err != nil {
			return err
		}
		if err := requireMemberGrant(tx, approved.Access, storeID); err != nil {
			return err
		}
		var stored workbenchStoreRecord
		if err := tx.Where("organization_id = ? AND id = ? AND deleted_at IS NULL", subject.OrganizationID, storeID).Take(&stored).Error; err != nil {
			return err
		}
		if stored.Platform != "shein" || stored.RecordStatus != "active" || stored.ServiceStatus == nil || *stored.ServiceStatus != "active" || stored.ServiceStartedAt == nil || stored.ServiceExpiresAt == nil || now.Before(*stored.ServiceStartedAt) || !now.Before(*stored.ServiceExpiresAt) || stored.ConnectionRef == "" {
			return ErrNotFound
		}
		var connection officialConnectionRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ? AND attempt_id = ?", subject.OrganizationID, storeID, stored.ConnectionRef).Take(&connection).Error; err != nil {
			return err
		}
		if connection.Status != "connected" {
			return ErrNotFound
		}
		var attempt officialAttemptRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ? AND attempt_id = ? AND connection_version = ?", subject.OrganizationID, storeID, connection.AttemptID, connection.Version).Take(&attempt).Error; err != nil {
			return err
		}
		if attempt.State != "verified" || attempt.KeyID == "" || attempt.Ciphertext == "" {
			return ErrNotFound
		}
		material = ProductExecutionMaterial{StoreVersion: stored.Version, Platform: Platform(stored.Platform), ServiceExpiresAt: stored.ServiceExpiresAt.UTC(), Connection: connectionView(connection, attempt), Attempt: attemptValue(attempt)}
		return ctx.Err()
	})
	if err != nil {
		return ProductExecutionMaterial{}, productExecutionReadError(err)
	}
	return material, nil
}

func productExecutionReadError(err error) error {
	if errors.Is(err, ErrNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return ErrDependencyUnavailable
}
