package storecenter

import (
	"context"
	"crypto/subtle"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type officialConnectionRow struct {
	OrganizationID string `gorm:"primaryKey;size:200;not null"`
	StoreID        string `gorm:"primaryKey;type:char(36);not null"`
	AttemptID      string `gorm:"type:char(36);not null"`
	Version        int64  `gorm:"not null"`
	Status         string `gorm:"size:32;not null"`
	ObservedAt     *time.Time
}

func (officialConnectionRow) TableName() string { return "workbench_store_connections" }

type officialAttemptRow struct {
	OrganizationID    string    `gorm:"primaryKey;size:200;not null"`
	AttemptID         string    `gorm:"primaryKey;type:char(36);not null"`
	StoreID           string    `gorm:"type:char(36);not null"`
	ActorID           string    `gorm:"size:200;not null"`
	MemberID          string    `gorm:"size:200;not null"`
	AppID             string    `gorm:"size:200;not null"`
	AppVersion        string    `gorm:"size:200;not null"`
	StateHash         string    `gorm:"size:64;not null"`
	StoreVersion      int64     `gorm:"not null"`
	ConnectionVersion int64     `gorm:"not null"`
	State             string    `gorm:"size:32;not null"`
	KeyID             string    `gorm:"size:128;not null"`
	Ciphertext        string    `gorm:"type:text;not null"`
	ExpiresAt         time.Time `gorm:"not null"`
	CreatedAt         time.Time `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
}

func (officialAttemptRow) TableName() string { return "workbench_store_connection_attempts" }

type officialMerchantBinding struct {
	AppID          string `gorm:"primaryKey;size:200;not null"`
	OpenKeyID      string `gorm:"primaryKey;size:200;not null"`
	OrganizationID string `gorm:"size:200;not null"`
	StoreID        string `gorm:"type:char(36);not null"`
}

func (officialMerchantBinding) TableName() string { return "workbench_store_merchant_bindings" }
func attemptValue(a officialAttemptRow) OfficialConnectionAttempt {
	return OfficialConnectionAttempt{OrganizationID: a.OrganizationID, StoreID: a.StoreID, AttemptID: a.AttemptID, ActorID: a.ActorID, MemberID: a.MemberID, AppID: a.AppID, AppVersion: a.AppVersion, StateHash: a.StateHash, StoreVersion: a.StoreVersion, ConnectionVersion: a.ConnectionVersion, ExpiresAt: a.ExpiresAt, State: a.State, KeyID: a.KeyID, Ciphertext: a.Ciphertext}
}
func connectionView(c officialConnectionRow, a officialAttemptRow) OfficialConnectionView {
	return OfficialConnectionView{AttemptID: c.AttemptID, State: a.State, Status: ConnectionStatus(c.Status), Version: c.Version, ObservedAt: c.ObservedAt}
}
func validConnectionCommand(c OfficialConnectionCommand) bool {
	return c.OrganizationID != "" && len(c.OrganizationID) <= MaxOrganizationIDBytes && uuid.Validate(c.StoreID) == nil && uuid.Validate(c.AttemptID) == nil && c.ExpectedStoreVersion > 0 && c.ExpectedStoreVersion < math.MaxInt64
}

// Begin fences the previous credential immediately. A new consent may rotate
// the remote secret, so the prior local credential must never be restored.
func (r *MemberScopedStoreRepository) BeginOfficialConnection(ctx context.Context, c OfficialConnectionCommand, app OfficialApplication, stateHash string, now time.Time) (result OfficialConnectionAttempt, err error) {
	if !validConnectionCommand(c) || app.AppID == "" || len(app.AppID) > 200 || app.Version == "" || len(app.Version) > 200 || len(stateHash) != 64 {
		return result, ErrNotFound
	}
	access, err := r.authorize(ctx, c.OrganizationID, true)
	if err != nil {
		return result, err
	}
	err = r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		store, err := connectionStore(tx, access, c.StoreID)
		if err != nil {
			return err
		}
		if store.Version != c.ExpectedStoreVersion {
			return ErrVersionConflict
		}
		var existing officialAttemptRow
		if err := tx.Where("organization_id = ? AND attempt_id = ?", c.OrganizationID, c.AttemptID).Take(&existing).Error; err == nil {
			return ErrAlreadyExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var current officialConnectionRow
		e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ?", c.OrganizationID, c.StoreID).Take(&current).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if current.Version == math.MaxInt64 {
			return ErrVersionConflict
		}
		if e == nil {
			if err := tx.Model(&officialAttemptRow{}).Where("organization_id = ? AND attempt_id = ?", c.OrganizationID, current.AttemptID).Updates(map[string]any{"state": "failed", "key_id": "", "ciphertext": "", "updated_at": now.UTC()}).Error; err != nil {
				return err
			}
		}
		next := officialConnectionRow{OrganizationID: c.OrganizationID, StoreID: c.StoreID, AttemptID: c.AttemptID, Version: current.Version + 1, Status: string(ConnectionStatusDisconnected)}
		if errors.Is(e, gorm.ErrRecordNotFound) {
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		} else if err := tx.Model(&officialConnectionRow{}).Where("organization_id = ? AND store_id = ?", c.OrganizationID, c.StoreID).Updates(map[string]any{"attempt_id": next.AttemptID, "version": next.Version, "status": next.Status, "observed_at": nil}).Error; err != nil {
			return err
		}
		if err := updateConnectionRef(tx, store, c.AttemptID, access.ActorID, now); err != nil {
			return err
		}
		row := officialAttemptRow{OrganizationID: c.OrganizationID, StoreID: c.StoreID, AttemptID: c.AttemptID, ActorID: access.ActorID, MemberID: access.MemberID, AppID: app.AppID, AppVersion: app.Version, StateHash: stateHash, StoreVersion: store.Version + 1, ConnectionVersion: next.Version, State: "awaiting_consent", ExpiresAt: now.UTC().Add(5 * time.Minute), CreatedAt: now.UTC(), UpdatedAt: now.UTC()}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		result = attemptValue(row)
		return nil
	})
	return result, err
}
func connectionStore(tx *gorm.DB, a StoreMemberAccess, id string) (workbenchStoreRecord, error) {
	var store workbenchStoreRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND id = ? AND deleted_at IS NULL AND record_status IN ?", a.OrganizationID, id, []string{string(RecordStatusActive), string(RecordStatusDisabled)}).Take(&store).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrNotFound
	}
	if err != nil {
		return store, err
	}
	if err := requireMemberGrant(tx, a, id); err != nil {
		return store, err
	}
	return store, nil
}
func updateConnectionRef(tx *gorm.DB, s workbenchStoreRecord, ref, actor string, now time.Time) error {
	if s.Version == math.MaxInt64 {
		return ErrVersionConflict
	}
	at := now.UTC()
	if at.Before(s.UpdatedAt) {
		at = s.UpdatedAt
	}
	update := tx.Model(&workbenchStoreRecord{}).Where("organization_id = ? AND id = ? AND version = ?", s.OrganizationID, s.ID, s.Version).Updates(map[string]any{"connection_ref": ref, "version": s.Version + 1, "updated_by": actor, "updated_at": at})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrVersionConflict
	}
	return nil
}
func (r *MemberScopedStoreRepository) authorizedAttempt(tx *gorm.DB, a StoreMemberAccess, attemptID string) (officialAttemptRow, officialConnectionRow, error) {
	var attempt officialAttemptRow
	// This first read only locates the Store. All writes lock Store -> grant ->
	// connection -> attempt consistently, including disconnect and completion.
	if err := tx.Where("organization_id = ? AND attempt_id = ?", a.OrganizationID, attemptID).Take(&attempt).Error; err != nil {
		return attempt, officialConnectionRow{}, ErrNotFound
	}
	store, err := connectionStore(tx, a, attempt.StoreID)
	if err != nil {
		return attempt, officialConnectionRow{}, err
	}
	var current officialConnectionRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ?", a.OrganizationID, attempt.StoreID).Take(&current).Error; err != nil {
		return attempt, current, ErrNotFound
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND attempt_id = ?", a.OrganizationID, attemptID).Take(&attempt).Error; err != nil {
		return attempt, current, err
	}
	if current.AttemptID != attemptID || current.Version != attempt.ConnectionVersion || store.ConnectionRef != attemptID || attempt.ActorID != a.ActorID || attempt.MemberID != a.MemberID || attempt.State == "failed" {
		return attempt, current, ErrNotFound
	}
	// A completed connection survives unrelated metadata updates; an unfinished
	// authorization may not apply against a changed Store record.
	if attempt.State != "verified" && store.Version != attempt.StoreVersion {
		return attempt, current, ErrVersionConflict
	}
	return attempt, current, nil
}
func (r *MemberScopedStoreRepository) ClaimOfficialExchange(ctx context.Context, org, storeID, attemptID, appID, appVersion, stateHash string, now time.Time) (result OfficialConnectionAttempt, dispatch bool, err error) {
	access, err := r.authorize(ctx, org, true)
	if err != nil {
		return result, false, err
	}
	err = r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		a, _, err := r.authorizedAttempt(tx, access, attemptID)
		if err != nil {
			return err
		}
		if a.StoreID != storeID || a.AppID != appID || a.AppVersion != appVersion || len(stateHash) != 64 || subtle.ConstantTimeCompare([]byte(a.StateHash), []byte(stateHash)) != 1 {
			return ErrNotFound
		}
		if a.State == "awaiting_consent" {
			if !now.Before(a.ExpiresAt) {
				return ErrOfficialAuthorizationRejected
			}
			if err := tx.Model(&officialAttemptRow{}).Where("organization_id = ? AND attempt_id = ? AND state = ?", org, attemptID, a.State).Updates(map[string]any{"state": "exchange_dispatched", "updated_at": now.UTC()}).Error; err != nil {
				return err
			}
			a.State = "exchange_dispatched"
			dispatch = true
		}
		result = attemptValue(a)
		return nil
	})
	if err != nil {
		dispatch = false
	}
	return result, dispatch, err
}
func (r *MemberScopedStoreRepository) SaveOfficialCredential(ctx context.Context, a OfficialConnectionAttempt, keyID, ciphertext, openKey string) error {
	if len(keyID) < 1 || len(keyID) > 128 || len(ciphertext) < 1 || len(ciphertext) > 16384 || len(openKey) < 1 || len(openKey) > 200 {
		return ErrOfficialExchangeUnknown
	}
	access, err := r.authorize(ctx, a.OrganizationID, true)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		original, _, err := r.authorizedAttempt(tx, access, a.AttemptID)
		if err != nil {
			return err
		}
		if attemptValue(original) != a || original.State != "exchange_dispatched" {
			return ErrNotFound
		}
		binding := officialMerchantBinding{AppID: a.AppID, OpenKeyID: openKey, OrganizationID: a.OrganizationID, StoreID: a.StoreID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&binding).Error; err != nil {
			return err
		}
		var existing officialMerchantBinding
		if err := tx.Where("app_id = ? AND open_key_id = ?", a.AppID, openKey).Take(&existing).Error; err != nil {
			return err
		}
		if existing.OrganizationID != a.OrganizationID || existing.StoreID != a.StoreID {
			return ErrAlreadyExists
		}
		return tx.Model(&officialAttemptRow{}).Where("organization_id = ? AND attempt_id = ? AND state = ?", a.OrganizationID, a.AttemptID, "exchange_dispatched").Updates(map[string]any{"state": "credential_received", "key_id": keyID, "ciphertext": ciphertext, "updated_at": time.Now().UTC()}).Error
	})
}
func (r *MemberScopedStoreRepository) CompleteOfficialConnection(ctx context.Context, a OfficialConnectionAttempt, status ConnectionStatus, now time.Time) (view OfficialConnectionView, err error) {
	access, err := r.authorize(ctx, a.OrganizationID, true)
	if err != nil {
		return view, err
	}
	if status != ConnectionStatusConnected && status != ConnectionStatusExpired && status != ConnectionStatusUnavailable && status != ConnectionStatusDisconnected {
		return view, ErrNotFound
	}
	err = r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		original, current, err := r.authorizedAttempt(tx, access, a.AttemptID)
		if err != nil {
			return err
		}
		if original.AppID != a.AppID || original.AppVersion != a.AppVersion || original.ConnectionVersion != a.ConnectionVersion {
			return ErrNotFound
		}
		next := original.State
		if status == ConnectionStatusConnected {
			if original.State != "credential_received" && original.State != "verified" {
				return ErrNotFound
			}
			next = "verified"
		} else if status == ConnectionStatusExpired || status == ConnectionStatusDisconnected {
			next = "failed"
		}
		updates := map[string]any{"state": next, "updated_at": now.UTC()}
		if next == "failed" {
			updates["key_id"] = ""
			updates["ciphertext"] = ""
		}
		if err := tx.Model(&officialAttemptRow{}).Where("organization_id = ? AND attempt_id = ?", a.OrganizationID, a.AttemptID).Updates(updates).Error; err != nil {
			return err
		}
		at := now.UTC()
		if err := tx.Model(&officialConnectionRow{}).Where("organization_id = ? AND store_id = ?", a.OrganizationID, a.StoreID).Updates(map[string]any{"status": string(status), "observed_at": at}).Error; err != nil {
			return err
		}
		original.State = next
		current.Status = string(status)
		current.ObservedAt = &at
		view = connectionView(current, original)
		return nil
	})
	return view, err
}
func (r *MemberScopedStoreRepository) ReadOfficialConnection(ctx context.Context, org, storeID string) (OfficialConnectionView, error) {
	if _, err := r.Get(ctx, org, storeID); err != nil {
		return OfficialConnectionView{}, err
	}
	var c officialConnectionRow
	err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Where("organization_id = ? AND store_id = ?", org, storeID).Take(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OfficialConnectionView{State: "disconnected", Status: ConnectionStatusDisconnected}, nil
	}
	if err != nil {
		return OfficialConnectionView{}, err
	}
	var a officialAttemptRow
	if err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Where("organization_id = ? AND attempt_id = ?", org, c.AttemptID).Take(&a).Error; err != nil {
		return OfficialConnectionView{}, err
	}
	return connectionView(c, a), nil
}

// Read/Observe are server-owned status boundaries. They require an exact
// current Store reference and never attach credentials or grant user access.
func (r *MemberScopedStoreRepository) ReadOfficialCredential(ctx context.Context, input ConnectionStatusInput) (OfficialConnectionAttempt, OfficialConnectionView, error) {
	if input.Platform != PlatformShein || input.OrganizationID == "" || uuid.Validate(input.StoreID) != nil {
		return OfficialConnectionAttempt{}, OfficialConnectionView{}, ErrNotFound
	}
	var s workbenchStoreRecord
	if err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Where("organization_id = ? AND id = ? AND deleted_at IS NULL", input.OrganizationID, input.StoreID).Take(&s).Error; err != nil || s.ConnectionRef != input.ConnectionRef {
		return OfficialConnectionAttempt{}, OfficialConnectionView{}, ErrNotFound
	}
	if input.ConnectionRef == "" {
		return OfficialConnectionAttempt{}, OfficialConnectionView{State: "disconnected", Status: ConnectionStatusDisconnected}, nil
	}
	var c officialConnectionRow
	var a officialAttemptRow
	if err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Where("organization_id = ? AND store_id = ? AND attempt_id = ?", input.OrganizationID, input.StoreID, input.ConnectionRef).Take(&c).Error; err != nil {
		return OfficialConnectionAttempt{}, OfficialConnectionView{}, ErrNotFound
	}
	if err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Where("organization_id = ? AND attempt_id = ? AND connection_version = ?", input.OrganizationID, c.AttemptID, c.Version).Take(&a).Error; err != nil {
		return OfficialConnectionAttempt{}, OfficialConnectionView{}, ErrNotFound
	}
	return attemptValue(a), connectionView(c, a), nil
}
func (r *MemberScopedStoreRepository) ObserveOfficialConnection(ctx context.Context, a OfficialConnectionAttempt, status ConnectionStatus, now time.Time) error {
	if status != ConnectionStatusConnected && status != ConnectionStatusExpired && status != ConnectionStatusUnavailable {
		return ErrNotFound
	}
	return r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		if err := lockMemberStore(tx, a.OrganizationID, a.StoreID); err != nil {
			return err
		}
		var current officialConnectionRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ? AND attempt_id = ? AND version = ?", a.OrganizationID, a.StoreID, a.AttemptID, a.ConnectionVersion).Take(&current).Error; err != nil {
			return ErrNotFound
		}
		var original officialAttemptRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND attempt_id = ?", a.OrganizationID, a.AttemptID).Take(&original).Error; err != nil {
			return err
		}
		if original.State != "verified" || original.KeyID != a.KeyID || original.Ciphertext != a.Ciphertext {
			return ErrNotFound
		}
		if status == ConnectionStatusExpired {
			if err := tx.Model(&officialAttemptRow{}).Where("organization_id = ? AND attempt_id = ?", a.OrganizationID, a.AttemptID).Updates(map[string]any{"state": "failed", "key_id": "", "ciphertext": "", "updated_at": now.UTC()}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&officialConnectionRow{}).Where("organization_id = ? AND store_id = ?", a.OrganizationID, a.StoreID).Updates(map[string]any{"status": string(status), "observed_at": now.UTC()}).Error
	})
}
func (r *MemberScopedStoreRepository) DisconnectOfficialConnection(ctx context.Context, c OfficialConnectionCommand, now time.Time) (view OfficialConnectionView, err error) {
	if !validConnectionCommand(c) {
		return view, ErrNotFound
	}
	access, err := r.authorize(ctx, c.OrganizationID, true)
	if err != nil {
		return view, err
	}
	err = r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Transaction(func(tx *gorm.DB) error {
		s, err := connectionStore(tx, access, c.StoreID)
		if err != nil {
			return err
		}
		if s.Version != c.ExpectedStoreVersion {
			return ErrVersionConflict
		}
		var current officialConnectionRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ?", c.OrganizationID, c.StoreID).Take(&current).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			view = OfficialConnectionView{State: "disconnected", Status: ConnectionStatusDisconnected}
			return nil
		} else if err != nil {
			return err
		}
		if current.Version == math.MaxInt64 {
			return ErrVersionConflict
		}
		if err := tx.Model(&officialAttemptRow{}).Where("organization_id = ? AND attempt_id = ?", c.OrganizationID, current.AttemptID).Updates(map[string]any{"state": "failed", "key_id": "", "ciphertext": "", "updated_at": now.UTC()}).Error; err != nil {
			return err
		}
		at := now.UTC()
		if err := tx.Model(&officialConnectionRow{}).Where("organization_id = ? AND store_id = ?", c.OrganizationID, c.StoreID).Updates(map[string]any{"version": current.Version + 1, "status": string(ConnectionStatusDisconnected), "observed_at": at}).Error; err != nil {
			return err
		}
		if err := updateConnectionRef(tx, s, "", access.ActorID, now); err != nil {
			return err
		}
		view = OfficialConnectionView{AttemptID: current.AttemptID, State: "failed", Status: ConnectionStatusDisconnected, Version: current.Version + 1, ObservedAt: &at}
		return nil
	})
	return view, err
}
