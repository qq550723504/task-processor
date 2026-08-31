package storecenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAuditIdentityMismatch = errors.New("store audit identity mismatch")

type AuditAction string

const (
	AuditActionQuotaReserved          AuditAction = "quota_reserved"
	AuditActionStoreCreated           AuditAction = "store_created"
	AuditActionQuotaCommitStarted     AuditAction = "quota_commit_started"
	AuditActionQuotaCommitFailed      AuditAction = "quota_commit_failed"
	AuditActionStoreCreateFailed      AuditAction = "store_create_failed"
	AuditActionStoreCreateUnknown     AuditAction = "store_create_unknown"
	AuditActionStoreCreationCommitted AuditAction = "store_creation_committed"
	AuditActionStoreUpdateStarted     AuditAction = "store_update_started"
	AuditActionStoreUpdated           AuditAction = "store_updated"
	AuditActionStoreUpdateNoOp        AuditAction = "store_update_noop"
	AuditActionStoreDisabled          AuditAction = "store_disabled"
	AuditActionStoreEnabled           AuditAction = "store_enabled"
	AuditActionDeleteStarted          AuditAction = "delete_started"
	AuditActionStoreMarkedDeleting    AuditAction = "store_marked_deleting"
	AuditActionQuotaDeallocated       AuditAction = "quota_deallocated"
	AuditActionDeleteComplete         AuditAction = "delete_complete"
)

type AuditOutcome string

const (
	AuditOutcomeSucceeded AuditOutcome = "succeeded"
	AuditOutcomeFailed    AuditOutcome = "failed"
	AuditOutcomeUnknown   AuditOutcome = "unknown"
)

type AuditFailureCode string

const (
	AuditFailureNone                  AuditFailureCode = ""
	AuditFailureAlreadyExists         AuditFailureCode = "already_exists"
	AuditFailureDependencyUnavailable AuditFailureCode = "dependency_unavailable"
)

// AuditEvent is the allowlisted, durable mutation trail. It intentionally has
// no extensible payload: provider/authentication material must never cross
// this boundary.
type AuditEvent struct {
	EventID        string           `json:"eventId"`
	OrganizationID string           `json:"-"`
	StoreID        string           `json:"storeId"`
	AllocationID   string           `json:"allocationId"`
	RequestKey     string           `json:"requestKey"`
	Action         AuditAction      `json:"action"`
	Outcome        AuditOutcome     `json:"outcome"`
	ActorSubject   string           `json:"actorSubject"`
	SafeFieldNames []string         `json:"safeFieldNames"`
	PreviousState  LifecycleStatus  `json:"previousState"`
	NewState       LifecycleStatus  `json:"newState"`
	FailureCode    AuditFailureCode `json:"failureCode"`
	StoreVersion   int64            `json:"storeVersion"`
	OccurredAt     time.Time        `json:"occurredAt"`
}

// AuditRepository is independent from the Store repository so composition can
// use one SQL handle while service orchestration stays persistence-agnostic.
type AuditRepository interface {
	Record(context.Context, AuditEvent) (event AuditEvent, replayed bool, err error)
	Get(ctx context.Context, organizationID, requestKey string, action AuditAction) (*AuditEvent, error)
}

type GormAuditRepository struct{ db *gorm.DB }

type workbenchStoreAuditLogRecord struct {
	EventID        string    `gorm:"column:event_id;type:char(36);primaryKey;not null"`
	OrganizationID string    `gorm:"column:organization_id;size:200;not null;index:idx_workbench_store_audit_org_store_created,priority:1;uniqueIndex:ux_workbench_store_audit_org_request_action,priority:1"`
	StoreID        string    `gorm:"column:store_id;type:char(36);not null;index:idx_workbench_store_audit_org_store_created,priority:2"`
	AllocationID   string    `gorm:"column:allocation_id;type:char(36);not null"`
	RequestKey     string    `gorm:"column:request_key;type:char(36);not null;uniqueIndex:ux_workbench_store_audit_org_request_action,priority:2"`
	Action         string    `gorm:"column:action;size:64;not null;uniqueIndex:ux_workbench_store_audit_org_request_action,priority:3"`
	Outcome        string    `gorm:"column:outcome;size:32;not null"`
	ActorSubject   string    `gorm:"column:actor_subject;size:200;not null"`
	SafeFieldNames string    `gorm:"column:safe_field_names;not null"`
	PreviousState  string    `gorm:"column:previous_state;size:32;not null"`
	NewState       string    `gorm:"column:new_state;size:32;not null"`
	FailureCode    string    `gorm:"column:failure_code;size:64;not null"`
	StoreVersion   int64     `gorm:"column:store_version;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;not null;index:idx_workbench_store_audit_org_store_created,priority:3"`
	OccurredAt     time.Time `gorm:"column:occurred_at;not null"`
}

func (workbenchStoreAuditLogRecord) TableName() string { return "workbench_store_audit_logs" }

func AutoMigrateAuditRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("store audit database is required")
	}
	return db.AutoMigrate(&workbenchStoreAuditLogRecord{})
}

func NewGormAuditRepository(db *gorm.DB) (*GormAuditRepository, error) {
	if db == nil {
		return nil, errors.New("store audit database is required")
	}
	return &GormAuditRepository{db: db}, nil
}

var _ AuditRepository = (*GormAuditRepository)(nil)

func (r *GormAuditRepository) Record(ctx context.Context, event AuditEvent) (AuditEvent, bool, error) {
	normalized, err := normalizeAuditEvent(event)
	if err != nil {
		return AuditEvent{}, false, err
	}
	record, err := auditRecordFromEvent(normalized)
	if err != nil {
		return AuditEvent{}, false, err
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "request_key"}, {Name: "action"}},
		DoNothing: true,
	}).Create(&record)
	if result.Error == nil && result.RowsAffected == 1 {
		return normalized, false, nil
	}
	existing, err := r.Get(ctx, normalized.OrganizationID, normalized.RequestKey, normalized.Action)
	if err != nil {
		return AuditEvent{}, false, fmt.Errorf("record store audit: %w", err)
	}
	if !sameAuditSemanticPayload(*existing, normalized) {
		return AuditEvent{}, false, ErrAuditIdentityMismatch
	}
	return *existing, true, nil
}

func (r *GormAuditRepository) Get(ctx context.Context, organizationID, requestKey string, action AuditAction) (*AuditEvent, error) {
	organizationID, err := validateOpaqueIdentity("organization ID", organizationID, MaxOrganizationIDBytes)
	if err != nil {
		return nil, err
	}
	if _, err := canonicalUUID(requestKey); err != nil {
		return nil, fmt.Errorf("request key: %w", err)
	}
	if !validAuditAction(action) {
		return nil, errors.New("audit action is invalid")
	}
	var record workbenchStoreAuditLogRecord
	err = r.db.WithContext(ctx).Where("organization_id = ? AND request_key = ? AND action = ?", organizationID, requestKey, string(action)).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get store audit: %w", err)
	}
	event, err := auditEventFromRecord(record)
	if err != nil {
		return nil, fmt.Errorf("rehydrate store audit: %w", err)
	}
	return &event, nil
}

func normalizeAuditEvent(event AuditEvent) (AuditEvent, error) {
	var err error
	if event.EventID, err = canonicalUUID(event.EventID); err != nil {
		return AuditEvent{}, fmt.Errorf("audit event ID: %w", err)
	}
	if event.OrganizationID, err = validateOpaqueIdentity("organization ID", event.OrganizationID, MaxOrganizationIDBytes); err != nil {
		return AuditEvent{}, err
	}
	if event.StoreID, err = canonicalUUID(event.StoreID); err != nil {
		return AuditEvent{}, fmt.Errorf("audit store ID: %w", err)
	}
	if event.AllocationID, err = canonicalUUID(event.AllocationID); err != nil {
		return AuditEvent{}, fmt.Errorf("audit allocation ID: %w", err)
	}
	if event.RequestKey, err = canonicalUUID(event.RequestKey); err != nil {
		return AuditEvent{}, fmt.Errorf("audit request key: %w", err)
	}
	if event.ActorSubject, err = validateOpaqueIdentity("actor subject", event.ActorSubject, MaxSubjectBytes); err != nil {
		return AuditEvent{}, err
	}
	if !validAuditAction(event.Action) || !validAuditOutcome(event.Outcome) || !validAuditFailure(event.FailureCode) {
		return AuditEvent{}, errors.New("audit event contains an unallowlisted value")
	}
	if !validAuditState(event.PreviousState) || !validAuditState(event.NewState) {
		return AuditEvent{}, errors.New("audit state is invalid")
	}
	if event.OccurredAt.IsZero() {
		return AuditEvent{}, errors.New("audit occurrence time is required")
	}
	if event.StoreVersion < 0 {
		return AuditEvent{}, errors.New("audit store version is invalid")
	}
	fields, err := normalizeSafeFieldNames(event.SafeFieldNames)
	if err != nil {
		return AuditEvent{}, err
	}
	event.SafeFieldNames = fields
	if !validTaskFiveAuditCombination(event) {
		return AuditEvent{}, errors.New("audit action outcome, state, fields, or version combination is invalid")
	}
	return event, nil
}

func validTaskFiveAuditCombination(event AuditEvent) bool {
	succeeded := event.Outcome == AuditOutcomeSucceeded && event.FailureCode == AuditFailureNone && event.StoreVersion > 0
	unknown := event.Outcome == AuditOutcomeUnknown && event.FailureCode == AuditFailureNone && event.StoreVersion > 0
	sameMutableState := event.PreviousState == event.NewState && (event.PreviousState == StoreStatusActive || event.PreviousState == StoreStatusDisabled)
	profileFields := len(event.SafeFieldNames) > 0
	for _, field := range event.SafeFieldNames {
		profileFields = profileFields && (field == "name" || field == "region")
	}
	switch event.Action {
	case AuditActionStoreUpdateStarted:
		return unknown && sameMutableState && profileFields
	case AuditActionStoreUpdated:
		return succeeded && sameMutableState && profileFields
	case AuditActionStoreUpdateNoOp:
		return succeeded && sameMutableState && len(event.SafeFieldNames) == 0
	case AuditActionStoreDisabled:
		return succeeded && event.PreviousState == StoreStatusActive && event.NewState == StoreStatusDisabled && exactSafeFields(event.SafeFieldNames, "lifecycle_status")
	case AuditActionStoreEnabled:
		return succeeded && event.PreviousState == StoreStatusDisabled && event.NewState == StoreStatusActive && exactSafeFields(event.SafeFieldNames, "lifecycle_status")
	case AuditActionDeleteStarted:
		return unknown && (event.PreviousState == StoreStatusActive || event.PreviousState == StoreStatusDisabled) && event.NewState == StoreStatusDeleting && exactSafeFields(event.SafeFieldNames, "lifecycle_status")
	case AuditActionStoreMarkedDeleting:
		return succeeded && (event.PreviousState == StoreStatusActive || event.PreviousState == StoreStatusDisabled) && event.NewState == StoreStatusDeleting && exactSafeFields(event.SafeFieldNames, "lifecycle_status")
	case AuditActionQuotaDeallocated:
		return succeeded && event.PreviousState == StoreStatusDeleting && event.NewState == StoreStatusDeleting && exactSafeFields(event.SafeFieldNames, "quota_allocation_id")
	case AuditActionDeleteComplete:
		return succeeded && event.PreviousState == StoreStatusDeleting && event.NewState == "" && exactSafeFields(event.SafeFieldNames, "lifecycle_status")
	default:
		return true
	}
}

func normalizeSafeFieldNames(fields []string) ([]string, error) {
	allowed := map[string]bool{"name": true, "platform": true, "region": true, "external_store_id": true, "lifecycle_status": true, "quota_allocation_id": true}
	set := make(map[string]bool, len(fields))
	for _, field := range fields {
		if !allowed[field] {
			return nil, errors.New("audit safe field name is invalid")
		}
		set[field] = true
	}
	out := make([]string, 0, len(set))
	for field := range set {
		out = append(out, field)
	}
	sort.Strings(out)
	return out, nil
}

func validAuditAction(action AuditAction) bool {
	switch action {
	case AuditActionQuotaReserved, AuditActionStoreCreated, AuditActionQuotaCommitStarted, AuditActionQuotaCommitFailed, AuditActionStoreCreateFailed, AuditActionStoreCreateUnknown, AuditActionStoreCreationCommitted, AuditActionStoreUpdateStarted, AuditActionStoreUpdated, AuditActionStoreUpdateNoOp, AuditActionStoreDisabled, AuditActionStoreEnabled, AuditActionDeleteStarted, AuditActionStoreMarkedDeleting, AuditActionQuotaDeallocated, AuditActionDeleteComplete:
		return true
	}
	return false
}
func validAuditOutcome(outcome AuditOutcome) bool {
	return outcome == AuditOutcomeSucceeded || outcome == AuditOutcomeFailed || outcome == AuditOutcomeUnknown
}
func validAuditFailure(code AuditFailureCode) bool {
	return code == AuditFailureNone || code == AuditFailureAlreadyExists || code == AuditFailureDependencyUnavailable
}
func validAuditState(state LifecycleStatus) bool { return state == "" || validLifecycleStatus(state) }

func auditRecordFromEvent(event AuditEvent) (workbenchStoreAuditLogRecord, error) {
	fields, err := json.Marshal(event.SafeFieldNames)
	if err != nil {
		return workbenchStoreAuditLogRecord{}, err
	}
	return workbenchStoreAuditLogRecord{EventID: event.EventID, OrganizationID: event.OrganizationID, StoreID: event.StoreID, AllocationID: event.AllocationID, RequestKey: event.RequestKey, Action: string(event.Action), Outcome: string(event.Outcome), ActorSubject: event.ActorSubject, SafeFieldNames: string(fields), PreviousState: string(event.PreviousState), NewState: string(event.NewState), FailureCode: string(event.FailureCode), StoreVersion: event.StoreVersion, CreatedAt: time.Now().UTC(), OccurredAt: event.OccurredAt.UTC()}, nil
}

func auditEventFromRecord(record workbenchStoreAuditLogRecord) (AuditEvent, error) {
	var fields []string
	if err := json.Unmarshal([]byte(record.SafeFieldNames), &fields); err != nil {
		return AuditEvent{}, err
	}
	return normalizeAuditEvent(AuditEvent{EventID: record.EventID, OrganizationID: record.OrganizationID, StoreID: record.StoreID, AllocationID: record.AllocationID, RequestKey: record.RequestKey, Action: AuditAction(record.Action), Outcome: AuditOutcome(record.Outcome), ActorSubject: record.ActorSubject, SafeFieldNames: fields, PreviousState: LifecycleStatus(record.PreviousState), NewState: LifecycleStatus(record.NewState), FailureCode: AuditFailureCode(record.FailureCode), StoreVersion: record.StoreVersion, OccurredAt: record.OccurredAt})
}

func sameAuditSemanticPayload(a, b AuditEvent) bool {
	a.EventID, b.EventID = "", ""
	a.OccurredAt, b.OccurredAt = time.Time{}, time.Time{}
	a.ActorSubject, b.ActorSubject = "", ""
	a.SafeFieldNames = append([]string(nil), a.SafeFieldNames...)
	b.SafeFieldNames = append([]string(nil), b.SafeFieldNames...)
	return reflect.DeepEqual(a, b)
}

func newAuditEvent(organizationID, storeID, allocationID, requestKey string, action AuditAction, outcome AuditOutcome, actor string, fields []string, previous, next LifecycleStatus, failure AuditFailureCode, occurredAt time.Time) AuditEvent {
	return AuditEvent{EventID: uuid.NewString(), OrganizationID: organizationID, StoreID: storeID, AllocationID: allocationID, RequestKey: requestKey, Action: action, Outcome: outcome, ActorSubject: actor, SafeFieldNames: fields, PreviousState: previous, NewState: next, FailureCode: failure, OccurredAt: occurredAt.UTC()}
}

func auditFailureFor(err error) AuditFailureCode {
	if errors.Is(err, ErrAlreadyExists) {
		return AuditFailureAlreadyExists
	}
	return AuditFailureDependencyUnavailable
}

func stableFailureFromAudit(event AuditEvent) error {
	switch event.FailureCode {
	case AuditFailureAlreadyExists:
		return ErrAlreadyExists
	default:
		return ErrDependencyUnavailable
	}
}

func auditEventMatchesAllocation(event AuditEvent, allocationID, storeID string) bool {
	return event.AllocationID == allocationID && event.StoreID == storeID
}
