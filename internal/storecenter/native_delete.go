package storecenter

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GormStoreRepository) DeleteRecord(ctx context.Context, request DeleteStoreRequest, at time.Time) (result DeleteStoreResult, err error) {
	request, err = normalizeDeleteStoreRequest(request)
	if err != nil || at.IsZero() {
		if err == nil {
			err = ErrInvalidTransition
		}
		return
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record workbenchStoreRecord
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id=? AND id=?", request.OrganizationID, request.StoreID).Take(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		fingerprint := hashTuple("native-store-delete", request.StoreID, strconv.FormatInt(request.ExpectedVersion, 10))
		audit := &GormAuditRepository{db: tx}
		if record.DeletedAt.Valid {
			event, err := audit.Get(ctx, request.OrganizationID, request.OperationKey, AuditActionDeleteComplete)
			if err != nil || event.StoreID != request.StoreID || event.ActorSubject != request.ActorSubject || event.PayloadFingerprint != fingerprint || record.DeleteOperationKey != request.OperationKey {
				return ErrInvalidTransition
			}
			result = DeleteStoreResult{StoreID: record.ID, Version: event.StoreVersion, Replayed: true}
			return nil
		}
		if record.Version != request.ExpectedVersion {
			return ErrVersionConflict
		}
		store, err := rehydrateRecord(record)
		if err != nil {
			return err
		}
		previous := store.RecordStatus()
		if at.Before(store.UpdatedAt()) {
			at = store.UpdatedAt()
		}
		if err := store.BeginDelete(request.OperationKey, request.ActorSubject, at); err != nil {
			return err
		}
		base := &GormStoreRepository{db: tx}
		if err := base.Save(ctx, request.OrganizationID, store, request.ExpectedVersion); err != nil {
			return err
		}
		if err := base.SoftDelete(ctx, request.OrganizationID, store.ID(), store.Version()); err != nil {
			return err
		}
		event := newAuditEvent(request.OrganizationID, store.ID(), request.OperationKey, AuditActionDeleteComplete, AuditOutcomeSucceeded, request.ActorSubject, []string{"record_status"}, previous, RecordStatusDeleted, AuditFailureNone, at)
		event.StoreVersion = store.Version() + 1
		event.PayloadFingerprint = fingerprint
		if _, _, err := audit.Record(ctx, event); err != nil {
			return err
		}
		result = DeleteStoreResult{StoreID: store.ID(), Version: event.StoreVersion}
		return nil
	})
	return result, err
}

func (r *MemberScopedStoreRepository) DeleteRecord(ctx context.Context, request DeleteStoreRequest, at time.Time) (result DeleteStoreResult, err error) {
	request, err = normalizeDeleteStoreRequest(request)
	if err != nil {
		return
	}
	access, err := r.authorize(ctx, request.OrganizationID, true)
	if err != nil {
		return
	}
	if access.ActorID != request.ActorSubject {
		return DeleteStoreResult{}, ErrNotFound
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record workbenchStoreRecord
		if err := tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id=? AND id=?", request.OrganizationID, request.StoreID).Take(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		base := &GormStoreRepository{db: tx}
		if record.DeletedAt.Valid {
			// Only read the original actor's durable receipt after live organization
			// authorization. Deletion revoked the grant in the original transaction.
			if !access.Administrator {
				event, receiptErr := (&GormAuditRepository{db: tx}).Get(ctx, request.OrganizationID, request.OperationKey, AuditActionDeleteComplete)
				if receiptErr != nil || event.ActorSubject != access.ActorID {
					return ErrNotFound
				}
			}
			var replayErr error
			result, replayErr = base.DeleteRecord(ctx, request, at)
			return replayErr
		}
		if err := requireMemberGrant(tx, access, request.StoreID); err != nil {
			return err
		}
		var err error
		result, err = base.DeleteRecord(ctx, request, at)
		if err != nil || result.Replayed {
			return err
		}
		store, err := rehydrateRecord(record)
		if err != nil {
			return err
		}
		store.deleteOperationKey = request.OperationKey
		store.updatedBy = request.ActorSubject
		return revokeStoreGrants(tx, request.OrganizationID, request.StoreID, store, at)
	})
	return result, err
}
