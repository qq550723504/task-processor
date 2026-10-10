package storecenter

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MemberID is the current identity provider's membership ID, not the user ID.
// The application supplies this capability after current Organization access.
type StoreMemberAccess struct {
	OrganizationID string
	ActorID        string
	MemberID       string
	Administrator  bool
	CanWrite       bool
}
type StoreMemberAuthorizer interface {
	AuthorizeStoreMember(context.Context, string) (StoreMemberAccess, error)
}

type storeMemberGrantRow struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey;size:200;not null"`
	StoreID        string    `gorm:"column:store_id;primaryKey;type:char(36);not null"`
	MemberID       string    `gorm:"column:member_id;primaryKey;size:200;not null"`
	Active         bool      `gorm:"column:active;not null"`
	Version        int64     `gorm:"column:version;not null;check:store_member_grant_version,version > 0"`
	UpdatedBy      string    `gorm:"column:updated_by;size:200;not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null"`
}

func (storeMemberGrantRow) TableName() string { return "workbench_store_member_grants" }

type storeMemberGrantOperation struct {
	OrganizationID  string    `gorm:"column:organization_id;primaryKey;size:200;not null"`
	OperationID     string    `gorm:"column:operation_id;primaryKey;type:char(36);not null"`
	StoreID         string    `gorm:"column:store_id;type:char(36);not null"`
	MemberID        string    `gorm:"column:member_id;size:200;not null"`
	Active          bool      `gorm:"column:active;not null"`
	ExpectedVersion int64     `gorm:"column:expected_version;not null"`
	ResultVersion   int64     `gorm:"column:result_version;not null"`
	ActorID         string    `gorm:"column:actor_id;size:200;not null"`
	Fingerprint     string    `gorm:"column:fingerprint;size:64;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
}

func (storeMemberGrantOperation) TableName() string { return "workbench_store_member_grant_operations" }

type MemberStoreGrantCommand struct {
	OrganizationID, StoreID, MemberID, OperationID string
	Active                                         bool
	ExpectedVersion                                int64
}

// All current Store consumers use this repository. The lower-level repository
// remains a persistence implementation and receives a borrowed local transaction.
type MemberScopedStoreRepository struct {
	db     *gorm.DB
	access StoreMemberAuthorizer
}

func NewMemberScopedStoreRepository(db *gorm.DB, access StoreMemberAuthorizer) (*MemberScopedStoreRepository, error) {
	if db == nil || isNilDependency(access) {
		return nil, ErrDependencyUnavailable
	}
	return &MemberScopedStoreRepository{db: db, access: access}, nil
}

var _ Repository = (*MemberScopedStoreRepository)(nil)

func (r *MemberScopedStoreRepository) authorize(ctx context.Context, org string, write bool) (StoreMemberAccess, error) {
	access, err := r.access.AuthorizeStoreMember(ctx, org)
	if err != nil {
		return StoreMemberAccess{}, err
	}
	if access.OrganizationID != org {
		return StoreMemberAccess{}, ErrNotFound
	}
	if _, err := validateOpaqueIdentity("actor", access.ActorID, MaxSubjectBytes); err != nil {
		return StoreMemberAccess{}, ErrNotFound
	}
	if _, err := validateOpaqueIdentity("member", access.MemberID, MaxSubjectBytes); err != nil {
		return StoreMemberAccess{}, ErrNotFound
	}
	if write && !access.CanWrite {
		return StoreMemberAccess{}, ErrNotFound
	}
	return access, nil
}
func requireMemberGrant(tx *gorm.DB, access StoreMemberAccess, storeID string) error {
	if access.Administrator {
		return nil
	}
	var row storeMemberGrantRow
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ? AND member_id = ? AND active = ?", access.OrganizationID, storeID, access.MemberID, true).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
func lockMemberStore(tx *gorm.DB, org, id string) error {
	var row workbenchStoreRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND id = ? AND deleted_at IS NULL", org, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (r *MemberScopedStoreRepository) CreateOrReplay(ctx context.Context, org string, candidate *Store) (stored *Store, replayed bool, err error) {
	access, err := r.authorize(ctx, org, true)
	if err != nil {
		return nil, false, err
	}
	if candidate == nil {
		return nil, false, ErrNotFound
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		base, _ := NewGormStoreRepository(tx)
		var createErr error
		stored, replayed, createErr = base.CreateOrReplay(ctx, org, candidate)
		if createErr != nil {
			return createErr
		}
		if replayed {
			if err := lockMemberStore(tx, org, stored.ID()); err != nil {
				return err
			}
			return requireMemberGrant(tx, access, stored.ID())
		}
		if !access.Administrator {
			if candidate.CreatedBy() != access.ActorID {
				return ErrNotFound
			}
			if err := tx.Create(&storeMemberGrantRow{OrganizationID: org, StoreID: stored.ID(), MemberID: access.MemberID, Active: true, Version: 1, UpdatedBy: access.ActorID, UpdatedAt: stored.CreatedAt()}).Error; err != nil {
				return err
			}
			command := MemberStoreGrantCommand{OrganizationID: org, StoreID: stored.ID(), MemberID: access.MemberID, OperationID: stored.CreateIdempotencyKey(), Active: true}
			return recordMemberGrantOperation(tx, command, access.ActorID, 1, stored.CreatedAt())
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return stored, replayed, nil
}
func (r *MemberScopedStoreRepository) List(ctx context.Context, org string, query StoreListQuery) (StorePage, error) {
	access, err := r.authorize(ctx, org, false)
	if err != nil {
		return StorePage{}, err
	}
	if !access.Administrator {
		query.MemberID = access.MemberID
	}
	base, _ := NewGormStoreRepository(r.db)
	return base.List(ctx, org, query)
}
func (r *MemberScopedStoreRepository) Get(ctx context.Context, org, id string) (*Store, error) {
	access, err := r.authorize(ctx, org, false)
	if err != nil {
		return nil, err
	}
	if err := requireMemberGrant(r.db.WithContext(ctx), access, id); err != nil {
		return nil, err
	}
	base, _ := NewGormStoreRepository(r.db)
	return base.Get(ctx, org, id)
}

// LockRead fences a bounded set of Store records and member grants in the
// caller's borrowed transaction. Administrators also lock Store rows. The
// current owner keeps the same Store-then-grant order as its mutations.
func (r *MemberScopedStoreRepository) LockRead(ctx context.Context, org string, ids []string) error {
	if r == nil || r.db == nil || r.db.Statement == nil {
		return ErrDependencyUnavailable
	}
	if _, borrowed := r.db.Statement.ConnPool.(gorm.TxCommitter); !borrowed {
		return ErrDependencyUnavailable
	}
	if len(ids) == 0 || len(ids) > 50 {
		return ErrNotFound
	}
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	for i, id := range sorted {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || parsed.String() != id || i > 0 && sorted[i-1] == id {
			return ErrNotFound
		}
	}
	access, err := r.authorize(ctx, org, false)
	if err != nil {
		return err
	}
	tx := r.db.WithContext(ctx)
	for _, id := range sorted {
		if err := lockMemberStore(tx, org, id); err != nil {
			return err
		}
		if err := requireMemberGrant(tx, access, id); err != nil {
			return err
		}
	}
	return nil
}
func (r *MemberScopedStoreRepository) withWrite(ctx context.Context, org, id string, mutation func(*GormStoreRepository) error) error {
	access, err := r.authorize(ctx, org, true)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockMemberStore(tx, org, id); err != nil {
			return err
		}
		if err := requireMemberGrant(tx, access, id); err != nil {
			return err
		}
		base, _ := NewGormStoreRepository(tx)
		return mutation(base)
	})
}
func (r *MemberScopedStoreRepository) Save(ctx context.Context, org string, store *Store, version int64) error {
	if store == nil {
		return ErrNotFound
	}
	return r.withWrite(ctx, org, store.ID(), func(base *GormStoreRepository) error { return base.Save(ctx, org, store, version) })
}
func (r *MemberScopedStoreRepository) SoftDelete(ctx context.Context, org, id string, version int64) error {
	return r.withWrite(ctx, org, id, func(base *GormStoreRepository) error {
		store, err := base.Get(ctx, org, id)
		if err != nil {
			return err
		}
		if err := base.SoftDelete(ctx, org, id, version); err != nil {
			return err
		}
		return revokeStoreGrants(base.db, org, id, store, time.Now().UTC())
	})
}

// Grant/revoke serializes with Store mutations by locking the Store first.
// Current commands check this grant in their Store transaction; revocation
// therefore fences later mutations without erasing original operation evidence.
func (r *MemberScopedStoreRepository) SetMemberGrant(ctx context.Context, command MemberStoreGrantCommand) error {
	org, id, member, active := command.OrganizationID, command.StoreID, command.MemberID, command.Active
	access, err := r.authorize(ctx, org, true)
	if err != nil {
		return err
	}
	if !access.Administrator {
		return ErrNotFound
	}
	if _, err := validateOpaqueIdentity("member", member, MaxSubjectBytes); err != nil {
		return err
	}
	if _, err := canonicalUUID(command.OperationID); err != nil || command.ExpectedVersion < 0 {
		return ErrVersionConflict
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockMemberStore(tx, org, id); err != nil {
			return err
		}
		var operation storeMemberGrantOperation
		err := tx.Where("organization_id=? AND operation_id=?", org, command.OperationID).Take(&operation).Error
		if err == nil {
			if operation.Fingerprint != memberGrantFingerprint(command) {
				return ErrAlreadyExists
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var row storeMemberGrantRow
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id = ? AND store_id = ? AND member_id = ?", org, id, member).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if command.ExpectedVersion != 0 {
				return ErrVersionConflict
			}
			at := time.Now().UTC()
			if err := tx.Create(&storeMemberGrantRow{OrganizationID: org, StoreID: id, MemberID: member, Active: active, Version: 1, UpdatedBy: access.ActorID, UpdatedAt: at}).Error; err != nil {
				return err
			}
			return recordMemberGrantOperation(tx, command, access.ActorID, 1, at)
		}
		if err != nil {
			return err
		}
		if row.Version != command.ExpectedVersion || row.Version == math.MaxInt64 {
			return ErrVersionConflict
		}
		at := time.Now().UTC()
		if row.Active != active {
			if err := tx.Model(&row).Updates(map[string]any{"active": active, "version": row.Version + 1, "updated_by": access.ActorID, "updated_at": at}).Error; err != nil {
				return err
			}
			row.Version++
		}
		return recordMemberGrantOperation(tx, command, access.ActorID, row.Version, at)
	})
}

func memberGrantFingerprint(command MemberStoreGrantCommand) string {
	return hashTuple("store-member-grant-v1", command.OrganizationID, command.StoreID, command.MemberID, strconv.FormatBool(command.Active), strconv.FormatInt(command.ExpectedVersion, 10))
}
func recordMemberGrantOperation(tx *gorm.DB, command MemberStoreGrantCommand, actor string, version int64, at time.Time) error {
	return tx.Create(&storeMemberGrantOperation{OrganizationID: command.OrganizationID, OperationID: command.OperationID, StoreID: command.StoreID, MemberID: command.MemberID, Active: command.Active, ExpectedVersion: command.ExpectedVersion, ResultVersion: version, ActorID: actor, Fingerprint: memberGrantFingerprint(command), CreatedAt: at}).Error
}

func revokeStoreGrants(tx *gorm.DB, org, id string, store *Store, at time.Time) error {
	var grants []storeMemberGrantRow
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("organization_id=? AND store_id=? AND active=?", org, id, true).Order("member_id").Find(&grants).Error; err != nil {
		return err
	}
	for _, grant := range grants {
		if grant.Version == math.MaxInt64 {
			return ErrVersionConflict
		}
		if err := tx.Model(&grant).Updates(map[string]any{"active": false, "version": grant.Version + 1, "updated_by": store.UpdatedBy(), "updated_at": at}).Error; err != nil {
			return err
		}
		key := uuid.NewSHA1(uuid.NameSpaceOID, []byte(store.DeleteOperationKey()+":"+grant.MemberID)).String()
		command := MemberStoreGrantCommand{OrganizationID: org, StoreID: id, MemberID: grant.MemberID, OperationID: key, ExpectedVersion: grant.Version}
		if err := recordMemberGrantOperation(tx, command, store.UpdatedBy(), grant.Version+1, at); err != nil {
			return err
		}
	}
	return nil
}
