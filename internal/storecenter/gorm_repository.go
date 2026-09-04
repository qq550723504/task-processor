package storecenter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GormStoreRepository is the SQL implementation of the Organization-scoped
// Store repository. The aggregate remains private; this record is its explicit
// persistence representation.
type GormStoreRepository struct {
	db *gorm.DB
}

type workbenchStoreRecord struct {
	ID                       string         `gorm:"column:id;type:char(36);primaryKey;not null"`
	OrganizationID           string         `gorm:"column:organization_id;size:200;not null;index:idx_workbench_stores_org_lifecycle_updated,priority:1;index:idx_workbench_stores_org_record_status_updated,priority:1;index:idx_workbench_stores_org_platform_region,priority:1;uniqueIndex:ux_workbench_stores_org_create_key,priority:1;uniqueIndex:ux_workbench_stores_org_identity_key,priority:1"`
	Name                     string         `gorm:"column:name;not null"`
	Platform                 string         `gorm:"column:platform;not null;index:idx_workbench_stores_org_platform_region,priority:2"`
	Region                   string         `gorm:"column:region;not null;index:idx_workbench_stores_org_platform_region,priority:3"`
	ExternalStoreID          string         `gorm:"column:external_store_id;not null"`
	LifecycleStatus          string         `gorm:"column:lifecycle_status;not null;index:idx_workbench_stores_org_lifecycle_updated,priority:2"`
	RecordStatus             *string        `gorm:"column:record_status;size:32;index:idx_workbench_stores_org_record_status_updated,priority:2"`
	ServiceStatus            *string        `gorm:"column:service_status;size:32"`
	ServiceStartedAt         *time.Time     `gorm:"column:service_started_at"`
	ServiceExpiresAt         *time.Time     `gorm:"column:service_expires_at"`
	ConnectionRef            string         `gorm:"column:connection_ref;not null"`
	QuotaAllocationID        string         `gorm:"column:quota_allocation_id;type:char(36);not null"`
	Version                  int64          `gorm:"column:version;not null"`
	CreatedBy                string         `gorm:"column:created_by;size:200;not null"`
	UpdatedBy                string         `gorm:"column:updated_by;size:200;not null"`
	CreatedAt                time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt                time.Time      `gorm:"column:updated_at;not null;index:idx_workbench_stores_org_lifecycle_updated,priority:3;index:idx_workbench_stores_org_record_status_updated,priority:3"`
	DeletedAt                gorm.DeletedAt `gorm:"column:deleted_at;index"`
	CreateIdempotencyKey     string         `gorm:"column:create_idempotency_key;type:char(36);not null;uniqueIndex:ux_workbench_stores_org_create_key,priority:2"`
	DeleteOperationKey       string         `gorm:"column:delete_operation_key;type:varchar(36);not null"`
	IdentityKey              string         `gorm:"column:identity_key;size:64;not null;uniqueIndex:ux_workbench_stores_org_identity_key,priority:2"`
	CreateRequestFingerprint string         `gorm:"column:create_request_fingerprint;size:64;not null"`
}

func (workbenchStoreRecord) TableName() string { return "workbench_stores" }

// AutoMigrateStoreRepository creates the Store Center table. It is safe to
// call repeatedly and rejects a nil handle instead of panicking at startup.
func AutoMigrateStoreRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("store repository database is required")
	}
	return db.AutoMigrate(&workbenchStoreRecord{})
}

func NewGormStoreRepository(db *gorm.DB) (*GormStoreRepository, error) {
	if db == nil {
		return nil, errors.New("store repository database is required")
	}
	return &GormStoreRepository{db: db}, nil
}

var _ Repository = (*GormStoreRepository)(nil)

func (r *GormStoreRepository) CreateOrReplay(ctx context.Context, organizationID string, store *Store) (*Store, bool, error) {
	if err := requireStoreScope(organizationID, store); err != nil {
		return nil, false, err
	}
	snapshot := store.Snapshot()
	if err := requirePristineCreateSnapshot(snapshot); err != nil {
		return nil, false, err
	}
	fingerprint := createRequestFingerprint(snapshot)
	if existing, found, err := r.findByCreateKey(ctx, organizationID, snapshot.CreateIdempotencyKey); err != nil {
		return nil, false, err
	} else if found {
		return replayOrConflict(existing, fingerprint)
	}

	record := recordFromSnapshot(snapshot, identityKey(snapshot), fingerprint)
	if err := r.db.WithContext(ctx).Create(&record).Error; err == nil {
		created, err := r.Get(ctx, organizationID, snapshot.ID)
		if err != nil {
			return nil, false, fmt.Errorf("reload created workbench store: %w", err)
		}
		return created, false, nil
	} else {
		resolved, replayed, resolveErr := r.resolveCreateCollision(ctx, organizationID, snapshot.CreateIdempotencyKey, record.IdentityKey, fingerprint)
		if resolveErr == nil {
			return resolved, replayed, nil
		}
		if errors.Is(resolveErr, ErrAlreadyExists) {
			return nil, false, ErrAlreadyExists
		}
		return nil, false, fmt.Errorf("create workbench store: %w", err)
	}
}

func (r *GormStoreRepository) List(ctx context.Context, organizationID string, query StoreListQuery) (StorePage, error) {
	page, pageSize, err := normalizeStorePage(query.Page, query.PageSize)
	if err != nil {
		return StorePage{}, err
	}
	var result StorePage
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		base := tx.Model(&workbenchStoreRecord{}).
			Where("organization_id = ?", organizationID).
			Where("deleted_at IS NULL")
		if query.Platform != "" {
			base = base.Where("platform = ?", string(query.Platform))
		}
		if query.Status != "" {
			base = base.Where("lifecycle_status = ?", string(query.Status))
		}
		var total int64
		if err := base.Count(&total).Error; err != nil {
			return fmt.Errorf("count workbench stores: %w", err)
		}
		var records []workbenchStoreRecord
		if err := base.Order("updated_at DESC").Order("id ASC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&records).Error; err != nil {
			return fmt.Errorf("list workbench stores: %w", err)
		}
		stores := make([]Store, 0, len(records))
		for _, record := range records {
			store, err := rehydrateRecord(record)
			if err != nil {
				return fmt.Errorf("rehydrate workbench store: %w", err)
			}
			stores = append(stores, *store)
		}
		result = StorePage{Stores: stores, Total: total}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return StorePage{}, err
	}
	return result, nil
}

func (r *GormStoreRepository) Get(ctx context.Context, organizationID string, storeID string) (*Store, error) {
	var record workbenchStoreRecord
	err := r.scopedActiveRecords(ctx, organizationID).Where("id = ?", storeID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get workbench store: %w", err)
	}
	store, err := rehydrateRecord(record)
	if err != nil {
		return nil, fmt.Errorf("rehydrate workbench store: %w", err)
	}
	return store, nil
}

func (r *GormStoreRepository) Save(ctx context.Context, organizationID string, store *Store, expectedVersion int64) error {
	if err := requireStoreScope(organizationID, store); err != nil {
		return err
	}
	snapshot := store.Snapshot()
	if snapshot.Version != expectedVersion+1 {
		return errors.New("store snapshot version must advance exactly once")
	}
	if _, err := RehydrateStore(snapshot); err != nil {
		return fmt.Errorf("invalid store snapshot: %w", err)
	}
	durableRecord, err := r.loadActiveRecord(ctx, organizationID, snapshot.ID)
	if err != nil {
		return err
	}
	durableStore, err := rehydrateRecord(durableRecord)
	if err != nil {
		return fmt.Errorf("rehydrate durable workbench store: %w", err)
	}
	if durableStore.Version() != expectedVersion {
		return ErrVersionConflict
	}
	if err := validateSaveSnapshot(durableStore.Snapshot(), snapshot); err != nil {
		return err
	}
	compatibilityState, err := compatibilityStateForSave(durableRecord, snapshot.LifecycleStatus)
	if err != nil {
		return err
	}
	updates := map[string]any{
		"name":                 snapshot.Name,
		"region":               snapshot.Region,
		"lifecycle_status":     string(snapshot.LifecycleStatus),
		"connection_ref":       snapshot.ConnectionRef,
		"version":              snapshot.Version,
		"updated_by":           snapshot.UpdatedBy,
		"updated_at":           snapshot.UpdatedAt,
		"identity_key":         identityKey(snapshot),
		"delete_operation_key": snapshot.DeleteOperationKey,
	}
	for column, value := range compatibilityState.columns() {
		updates[column] = value
	}
	result := r.scopedActiveRecords(ctx, organizationID).Where("id = ? AND version = ?", snapshot.ID, expectedVersion).Updates(updates)
	if result.Error != nil {
		if isUniqueConstraint(result.Error) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("save workbench store: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}
	return r.classifyAbsentVersionedRow(ctx, organizationID, snapshot.ID)
}

// LockServiceState loads the exact Organization-scoped Store through a caller
// supplied transaction. The organization-resource adapter owns that
// transaction so Store and Bucket/Event/Operation writes share one commit.
func (r *GormStoreRepository) LockServiceState(ctx context.Context, tx *gorm.DB, identity ServiceStoreIdentity) (ServiceStoreSnapshot, error) {
	if tx == nil {
		return ServiceStoreSnapshot{}, errors.New("store service transaction is required")
	}
	organizationID, err := validateOpaqueIdentity("organization ID", identity.OrganizationID, MaxOrganizationIDBytes)
	if err != nil {
		return ServiceStoreSnapshot{}, err
	}
	storeID, err := canonicalUUID(identity.StoreID)
	if err != nil {
		return ServiceStoreSnapshot{}, fmt.Errorf("store ID: %w", err)
	}
	var record workbenchStoreRecord
	err = tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("organization_id = ? AND id = ? AND deleted_at IS NULL", organizationID, storeID).
		Take(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ServiceStoreSnapshot{}, ErrNotFound
	}
	if err != nil {
		return ServiceStoreSnapshot{}, fmt.Errorf("lock workbench Store service state: %w", err)
	}
	state, mapped := expandedStateFromRecord(record)
	if !mapped {
		return ServiceStoreSnapshot{}, ErrInvalidServiceState
	}
	if err := ValidateStoreServiceState(state); err != nil {
		return ServiceStoreSnapshot{}, err
	}
	return ServiceStoreSnapshot{
		Identity:          ServiceStoreIdentity{OrganizationID: record.OrganizationID, StoreID: record.ID},
		QuotaAllocationID: record.QuotaAllocationID,
		ConnectionRef:     record.ConnectionRef,
		Version:           record.Version,
		UpdatedAt:         record.UpdatedAt,
		State:             copyServiceState(state),
	}, nil
}

// ApplyServiceState performs only the Store half of a Store+Resource unit of
// work and therefore requires the same caller-owned transaction used for the
// resource mutation. It advances the aggregate version exactly once.
func (r *GormStoreRepository) ApplyServiceState(ctx context.Context, tx *gorm.DB, mutation ServiceStoreMutation) error {
	if tx == nil {
		return errors.New("store service transaction is required")
	}
	organizationID, err := validateOpaqueIdentity("organization ID", mutation.Identity.OrganizationID, MaxOrganizationIDBytes)
	if err != nil {
		return err
	}
	storeID, err := canonicalUUID(mutation.Identity.StoreID)
	if err != nil {
		return fmt.Errorf("store ID: %w", err)
	}
	actor, err := validateOpaqueIdentity("actor subject", mutation.ActorSubject, MaxSubjectBytes)
	if err != nil {
		return err
	}
	if mutation.ExpectedVersion <= 0 || mutation.OccurredAt.IsZero() {
		return ErrVersionConflict
	}
	if mutation.State.RecordStatus != RecordStatusActive {
		return ErrInvalidServiceTransition
	}
	if err := ValidateStoreServiceState(mutation.State); err != nil {
		return err
	}
	updates := mutation.State.columns()
	updates["version"] = mutation.ExpectedVersion + 1
	updates["updated_by"] = actor
	updates["updated_at"] = mutation.OccurredAt.UTC()
	result := tx.WithContext(ctx).Model(&workbenchStoreRecord{}).
		Where("organization_id = ? AND id = ? AND deleted_at IS NULL AND version = ? AND connection_ref = ? AND updated_at <= ?",
			organizationID, storeID, mutation.ExpectedVersion, mutation.ExpectedConnectionRef, mutation.OccurredAt.UTC()).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("apply workbench Store service state: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var record workbenchStoreRecord
	lookupErr := tx.WithContext(ctx).Where("organization_id = ? AND id = ? AND deleted_at IS NULL", organizationID, storeID).Take(&record).Error
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if lookupErr != nil {
		return fmt.Errorf("classify Store service CAS: %w", lookupErr)
	}
	if record.ConnectionRef != mutation.ExpectedConnectionRef {
		return ErrConnectionSnapshotChanged
	}
	return ErrVersionConflict
}

func (r *GormStoreRepository) SoftDelete(ctx context.Context, organizationID string, storeID string, expectedVersion int64) error {
	durableRecord, err := r.loadActiveRecord(ctx, organizationID, storeID)
	if err != nil {
		return err
	}
	durableStore, err := rehydrateRecord(durableRecord)
	if err != nil {
		return fmt.Errorf("rehydrate durable workbench store: %w", err)
	}
	if durableStore.LifecycleStatus() != StoreStatusDeleting {
		return ErrInvalidTransition
	}
	if durableStore.Version() != expectedVersion {
		return ErrVersionConflict
	}
	now := time.Now().UTC()
	if now.Before(durableStore.UpdatedAt()) {
		now = durableStore.UpdatedAt()
	}
	result := r.scopedActiveRecords(ctx, organizationID).
		Where("id = ? AND version = ? AND lifecycle_status = ?", storeID, expectedVersion, string(StoreStatusDeleting)).
		Updates(map[string]any{
			"deleted_at":         now,
			"updated_at":         now,
			"version":            gorm.Expr("version + ?", 1),
			"record_status":      string(RecordStatusDeleted),
			"service_status":     nil,
			"service_started_at": nil,
			"service_expires_at": nil,
		})
	if result.Error != nil {
		return fmt.Errorf("soft delete workbench store: %w", result.Error)
	}
	if result.RowsAffected == 1 {
		return nil
	}

	var record workbenchStoreRecord
	err = r.scopedActiveRecords(ctx, organizationID).Where("id = ?", storeID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify soft delete: %w", err)
	}
	if record.LifecycleStatus != string(StoreStatusDeleting) {
		return ErrInvalidTransition
	}
	return ErrVersionConflict
}

func (r *GormStoreRepository) scopedActiveRecords(ctx context.Context, organizationID string) *gorm.DB {
	return r.scopedRecords(ctx, organizationID).Where("deleted_at IS NULL")
}

func (r *GormStoreRepository) scopedRecords(ctx context.Context, organizationID string) *gorm.DB {
	return r.db.WithContext(ctx).Model(&workbenchStoreRecord{}).Where("organization_id = ?", organizationID)
}

func (r *GormStoreRepository) findByCreateKey(ctx context.Context, organizationID, key string) (workbenchStoreRecord, bool, error) {
	var record workbenchStoreRecord
	err := r.scopedActiveRecords(ctx, organizationID).Where("create_idempotency_key = ?", key).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workbenchStoreRecord{}, false, nil
	}
	if err != nil {
		return workbenchStoreRecord{}, false, fmt.Errorf("find idempotent store request: %w", err)
	}
	return record, true, nil
}

func (r *GormStoreRepository) resolveCreateCollision(ctx context.Context, organizationID, key, identity, fingerprint string) (*Store, bool, error) {
	if record, found, err := r.findByCreateKey(ctx, organizationID, key); err != nil {
		return nil, false, err
	} else if found {
		return replayOrConflict(record, fingerprint)
	}
	var record workbenchStoreRecord
	err := r.scopedRecords(ctx, organizationID).Unscoped().Where("create_idempotency_key = ?", key).First(&record).Error
	if err == nil {
		return nil, false, ErrAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, fmt.Errorf("resolve deleted idempotent store request: %w", err)
	}
	err = r.scopedRecords(ctx, organizationID).Unscoped().Where("identity_key = ?", identity).First(&record).Error
	if err == nil {
		return nil, false, ErrAlreadyExists
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, errors.New("no same-organization uniqueness collision found")
	}
	return nil, false, fmt.Errorf("resolve store identity collision: %w", err)
}

func replayOrConflict(record workbenchStoreRecord, fingerprint string) (*Store, bool, error) {
	if record.CreateRequestFingerprint != fingerprint {
		return nil, false, ErrAlreadyExists
	}
	store, err := rehydrateRecord(record)
	if err != nil {
		return nil, false, fmt.Errorf("rehydrate replayed store: %w", err)
	}
	return store, true, nil
}

func (r *GormStoreRepository) classifyAbsentVersionedRow(ctx context.Context, organizationID, storeID string) error {
	var record workbenchStoreRecord
	err := r.scopedActiveRecords(ctx, organizationID).Where("id = ?", storeID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("classify versioned store write: %w", err)
	}
	return ErrVersionConflict
}

func (r *GormStoreRepository) loadActiveRecord(ctx context.Context, organizationID, storeID string) (workbenchStoreRecord, error) {
	var record workbenchStoreRecord
	err := r.scopedActiveRecords(ctx, organizationID).Where("id = ?", storeID).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return workbenchStoreRecord{}, ErrNotFound
	}
	if err != nil {
		return workbenchStoreRecord{}, fmt.Errorf("load workbench store: %w", err)
	}
	return record, nil
}

func recordFromSnapshot(snapshot StoreSnapshot, identity, fingerprint string) workbenchStoreRecord {
	record := workbenchStoreRecord{
		ID: snapshot.ID, OrganizationID: snapshot.OrganizationID, Name: snapshot.Name, Platform: string(snapshot.Platform), Region: snapshot.Region,
		ExternalStoreID: snapshot.ExternalStoreID, LifecycleStatus: string(snapshot.LifecycleStatus), ConnectionRef: snapshot.ConnectionRef,
		QuotaAllocationID: snapshot.QuotaAllocationID, Version: snapshot.Version, CreatedBy: snapshot.CreatedBy, UpdatedBy: snapshot.UpdatedBy,
		CreatedAt: snapshot.CreatedAt, UpdatedAt: snapshot.UpdatedAt, CreateIdempotencyKey: snapshot.CreateIdempotencyKey, DeleteOperationKey: snapshot.DeleteOperationKey,
		IdentityKey: identity, CreateRequestFingerprint: fingerprint,
	}
	record.applyCompatibilityState(compatibilityStateForNewStore(snapshot.LifecycleStatus))
	if snapshot.DeletedAt != nil {
		record.DeletedAt = gorm.DeletedAt{Time: *snapshot.DeletedAt, Valid: true}
	}
	return record
}

func compatibilityStateForNewStore(status LifecycleStatus) StoreServiceState {
	switch status {
	case StoreStatusProvisioning:
		return StoreServiceState{RecordStatus: RecordStatusProvisioning}
	case StoreStatusActive:
		return StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusPendingActivation}
	case StoreStatusDisabled:
		return StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusSuspended}
	case StoreStatusDeleting:
		return StoreServiceState{RecordStatus: RecordStatusDeleting}
	default:
		return StoreServiceState{}
	}
}

func compatibilityStateForSave(durable workbenchStoreRecord, incoming LifecycleStatus) (StoreServiceState, error) {
	state, mapped := expandedStateFromRecord(durable)
	if !mapped {
		state = StoreServiceState{}
	}

	switch incoming {
	case StoreStatusProvisioning:
		state = StoreServiceState{RecordStatus: RecordStatusProvisioning}
	case StoreStatusDeleting:
		state = StoreServiceState{RecordStatus: RecordStatusDeleting}
	case StoreStatusDisabled:
		state.RecordStatus = RecordStatusActive
		state.ServiceStatus = ServiceStatusSuspended
	case StoreStatusActive:
		state.RecordStatus = RecordStatusActive
		if state.ServiceStatus == ServiceStatusSuspended && validServicePeriod(state.StartedAt, state.ExpiresAt) {
			state.ServiceStatus = ServiceStatusActive
		} else if state.ServiceStatus != ServiceStatusActive && state.ServiceStatus != ServiceStatusExpired {
			state.ServiceStatus = ServiceStatusPendingActivation
		}
	default:
		return StoreServiceState{}, ErrInvalidServiceState
	}
	if err := ValidateStoreServiceState(state); err != nil {
		return StoreServiceState{}, err
	}
	return state, nil
}

func expandedStateFromRecord(record workbenchStoreRecord) (StoreServiceState, bool) {
	if record.RecordStatus == nil && record.ServiceStatus == nil && record.ServiceStartedAt == nil && record.ServiceExpiresAt == nil {
		return StoreServiceState{}, false
	}
	state := StoreServiceState{StartedAt: copyTimePointer(record.ServiceStartedAt), ExpiresAt: copyTimePointer(record.ServiceExpiresAt)}
	if record.RecordStatus != nil {
		state.RecordStatus = RecordStatus(*record.RecordStatus)
	}
	if record.ServiceStatus != nil {
		state.ServiceStatus = ServiceStatus(*record.ServiceStatus)
	}
	return state, true
}

func (record *workbenchStoreRecord) applyCompatibilityState(state StoreServiceState) {
	record.RecordStatus = optionalString(string(state.RecordStatus))
	record.ServiceStatus = optionalString(string(state.ServiceStatus))
	record.ServiceStartedAt = copyTimePointer(state.StartedAt)
	record.ServiceExpiresAt = copyTimePointer(state.ExpiresAt)
}

func (state StoreServiceState) columns() map[string]any {
	return map[string]any{
		"record_status":      optionalString(string(state.RecordStatus)),
		"service_status":     optionalString(string(state.ServiceStatus)),
		"service_started_at": copyTimePointer(state.StartedAt),
		"service_expires_at": copyTimePointer(state.ExpiresAt),
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func rehydrateRecord(record workbenchStoreRecord) (*Store, error) {
	snapshot := StoreSnapshot{
		ID: record.ID, OrganizationID: record.OrganizationID, Name: record.Name, Platform: Platform(record.Platform), Region: record.Region,
		ExternalStoreID: record.ExternalStoreID, LifecycleStatus: LifecycleStatus(record.LifecycleStatus), ConnectionRef: record.ConnectionRef,
		QuotaAllocationID: record.QuotaAllocationID, Version: record.Version, CreatedBy: record.CreatedBy, UpdatedBy: record.UpdatedBy,
		CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt, CreateIdempotencyKey: record.CreateIdempotencyKey, DeleteOperationKey: record.DeleteOperationKey,
	}
	if record.DeletedAt.Valid {
		deletedAt := record.DeletedAt.Time
		snapshot.DeletedAt = &deletedAt
	}
	return RehydrateStore(snapshot)
}

func requireStoreScope(organizationID string, store *Store) error {
	if store == nil {
		return errors.New("store is required")
	}
	if organizationID == "" || store.OrganizationID() != organizationID {
		return errors.New("store organization does not match repository scope")
	}
	return nil
}

func requirePristineCreateSnapshot(snapshot StoreSnapshot) error {
	if snapshot.LifecycleStatus != StoreStatusProvisioning || snapshot.Version != 1 || snapshot.DeletedAt != nil || snapshot.ConnectionRef != "" || snapshot.DeleteOperationKey != "" || snapshot.CreatedBy != snapshot.UpdatedBy || !snapshot.CreatedAt.Equal(snapshot.UpdatedAt) {
		return errors.New("store creation snapshot must be pristine provisioning state")
	}
	return nil
}

func validateSaveSnapshot(durable, incoming StoreSnapshot) error {
	if durable.ID != incoming.ID || durable.OrganizationID != incoming.OrganizationID || durable.Platform != incoming.Platform || durable.ExternalStoreID != incoming.ExternalStoreID || durable.ConnectionRef != incoming.ConnectionRef || durable.QuotaAllocationID != incoming.QuotaAllocationID || durable.CreateIdempotencyKey != incoming.CreateIdempotencyKey || durable.CreatedBy != incoming.CreatedBy || !durable.CreatedAt.Equal(incoming.CreatedAt) {
		return errors.New("store immutable fields changed")
	}
	if incoming.DeletedAt != nil {
		return errors.New("store deletion must use soft delete")
	}
	if incoming.UpdatedAt.Before(durable.UpdatedAt) {
		return errors.New("store update time must not precede durable update")
	}
	beginningDelete := (durable.LifecycleStatus == StoreStatusActive || durable.LifecycleStatus == StoreStatusDisabled) && incoming.LifecycleStatus == StoreStatusDeleting && durable.DeleteOperationKey == "" && incoming.DeleteOperationKey != ""
	profileChanged := durable.Name != incoming.Name || durable.Region != incoming.Region
	lifecycleChanged := incoming.LifecycleStatus != durable.LifecycleStatus
	if beginningDelete {
		if profileChanged {
			return errors.New("store profile cannot change while deletion begins")
		}
		return nil
	}
	if durable.DeleteOperationKey != incoming.DeleteOperationKey {
		return errors.New("store delete operation key changed")
	}
	if lifecycleChanged && profileChanged {
		return errors.New("store profile and lifecycle cannot change together")
	}
	if profileChanged {
		if durable.LifecycleStatus == StoreStatusActive || durable.LifecycleStatus == StoreStatusDisabled {
			return nil
		}
		return ErrInvalidTransition
	}
	if lifecycleChanged {
		if canTransition(durable.LifecycleStatus, incoming.LifecycleStatus) {
			return nil
		}
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, durable.LifecycleStatus, incoming.LifecycleStatus)
	}
	return errors.New("store save must change profile or lifecycle state")
}

func normalizeStorePage(page, pageSize int) (int, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page-1 > int(^uint(0)>>1)/pageSize {
		return 0, 0, ErrPageOffsetOverflow
	}
	return page, pageSize, nil
}

func identityKey(snapshot StoreSnapshot) string {
	if snapshot.ExternalStoreID == "" {
		return hashTuple("store-identity-empty-external", snapshot.ID)
	}
	return hashTuple("store-identity", string(snapshot.Platform), snapshot.Region, snapshot.ExternalStoreID)
}

func createRequestFingerprint(snapshot StoreSnapshot) string {
	return hashTuple("store-create-request", snapshot.ID, snapshot.QuotaAllocationID, string(snapshot.Platform), snapshot.Region, snapshot.ExternalStoreID, snapshot.Name)
}

func hashTuple(values ...string) string {
	hash := sha256.New()
	var length [8]byte
	for _, value := range values {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key")
}
