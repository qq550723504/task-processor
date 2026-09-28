package storecenter

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrServiceChargeUnknown = errors.New("store service operation outcome is unknown")

// These are Store-owned facts. Resource reservations and balances are owned by
// the separate Resource ledger; no ledger transaction enters this repository.
type ServiceChargeIntent struct {
	Execution ServiceExecution
	MemberID  string
	Funding   string
}
type ServiceChargeProof struct {
	Intent                                        ServiceChargeIntent
	ReservationID, State, EvidenceID, FailureCode string
	Snapshot                                      ServiceOperationSnapshot
}
type ServiceChargeStore interface {
	AdmitServiceCharge(context.Context, ServiceChargeIntent) (ServiceChargeIntent, error)
	BindServiceCharge(context.Context, ServiceChargeIntent, string) error
	ApplyServiceCharge(context.Context, ServiceChargeIntent, ConnectionStatus) (ServiceChargeProof, error)
	FenceServiceCharge(context.Context, ServiceChargeIntent, string, string) (ServiceChargeProof, error)
	ReadServiceChargeIntent(context.Context, string, string) (ServiceChargeIntent, error)
	ReadServiceChargeProof(context.Context, string, string) (ServiceChargeProof, error)
}

type storeServiceChargeRow struct {
	OrganizationID string    `gorm:"column:organization_id;primaryKey;size:200;not null"`
	OperationID    string    `gorm:"column:operation_id;primaryKey;type:char(36);not null"`
	StoreID        string    `gorm:"column:store_id;type:char(36);not null"`
	IntentJSON     string    `gorm:"column:intent_json;type:text;not null"`
	ReservationID  string    `gorm:"column:reservation_id;type:varchar(36);not null;default:''"`
	State          string    `gorm:"column:state;size:32;not null"`
	SnapshotJSON   string    `gorm:"column:snapshot_json;type:text;not null;default:''"`
	EvidenceID     string    `gorm:"column:evidence_id;size:128;not null;default:''"`
	FailureCode    string    `gorm:"column:failure_code;size:64;not null;default:''"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time `gorm:"column:updated_at;not null"`
}

func (storeServiceChargeRow) TableName() string { return "workbench_store_service_operations" }

func validServiceIntent(intent ServiceChargeIntent) bool {
	e := intent.Execution
	if _, err := canonicalUUID(e.OperationID); err != nil {
		return false
	}
	if _, err := canonicalUUID(e.StoreID); err != nil {
		return false
	}
	if _, err := validateOpaqueIdentity("member", intent.MemberID, MaxSubjectBytes); err != nil {
		return false
	}
	if _, err := validateOpaqueIdentity("actor", e.ActorSubject, MaxSubjectBytes); err != nil {
		return false
	}
	if _, err := validateOpaqueIdentity("organization", e.OrganizationID, MaxOrganizationIDBytes); err != nil {
		return false
	}
	if e.ExpectedStoreVersion < 1 || e.ExpectedStoreVersion == math.MaxInt64 || e.OccurredAt.IsZero() || len(e.RequestFingerprint) != 64 {
		return false
	}
	if _, err := hex.DecodeString(e.RequestFingerprint); err != nil {
		return false
	}
	if intent.Funding != "member_allocated" && intent.Funding != "enterprise_unallocated" {
		return false
	}
	maximum := Phase1MaxStoreServicePeriods
	if e.Command == ServiceCommandActivate {
		maximum = 1
	} else if e.Command != ServiceCommandRenew && e.Command != ServiceCommandReactivate {
		return false
	}
	return e.MaxQuantity == maximum && validateServiceQuantity(e.Quantity, maximum) == nil
}
func sameServiceChargeIntent(a, b ServiceChargeIntent) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func sameServiceChargeRequest(a, b ServiceChargeIntent) bool {
	x, y := a.Execution, b.Execution
	return a.MemberID == b.MemberID && x.ActorSubject == y.ActorSubject && x.OrganizationID == y.OrganizationID && x.OperationID == y.OperationID && x.StoreID == y.StoreID && x.Command == y.Command && x.Quantity == y.Quantity && x.ExpectedStoreVersion == y.ExpectedStoreVersion && x.RequestFingerprint == y.RequestFingerprint
}
func readServiceCharge(tx *gorm.DB, org, op string, lock bool) (storeServiceChargeRow, ServiceChargeProof, error) {
	var row storeServiceChargeRow
	query := tx.Where("organization_id=? AND operation_id=?", org, op)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return row, ServiceChargeProof{}, ErrNotFound
	} else if err != nil {
		return row, ServiceChargeProof{}, err
	}
	var intent ServiceChargeIntent
	if len(row.IntentJSON) > 16384 || json.Unmarshal([]byte(row.IntentJSON), &intent) != nil || !validServiceIntent(intent) || intent.Execution.OrganizationID != org || intent.Execution.OperationID != op || intent.Execution.StoreID != row.StoreID {
		return row, ServiceChargeProof{}, ErrServiceChargeUnknown
	}
	proof := ServiceChargeProof{Intent: intent, ReservationID: row.ReservationID, State: row.State, EvidenceID: row.EvidenceID, FailureCode: row.FailureCode}
	if row.State == "succeeded" {
		if len(row.SnapshotJSON) > 16384 || json.Unmarshal([]byte(row.SnapshotJSON), &proof.Snapshot) != nil || proof.Snapshot.OrganizationID != org || proof.Snapshot.OperationID != op || proof.Snapshot.StoreID != row.StoreID || proof.Snapshot.StoreVersion != intent.Execution.ExpectedStoreVersion+1 || proof.Snapshot.Quantity != strconv.FormatInt(intent.Execution.Quantity, 10) || ValidateStoreServiceState(proof.Snapshot.ServiceState) != nil || proof.ReservationID == "" || proof.EvidenceID == "" {
			return row, ServiceChargeProof{}, ErrServiceChargeUnknown
		}
	}
	return row, proof, nil
}

func (r *MemberScopedStoreRepository) ServiceMemberAccess(ctx context.Context, org, id string) (StoreMemberAccess, error) {
	access, err := r.authorize(ctx, org, true)
	if err != nil {
		return access, err
	}
	if _, err := r.Get(ctx, org, id); err != nil {
		return access, err
	}
	return access, nil
}
func (r *MemberScopedStoreRepository) ReadServiceChargeIntent(ctx context.Context, org, op string) (ServiceChargeIntent, error) {
	_, proof, err := readServiceCharge(r.db.WithContext(ctx), org, op, false)
	return proof.Intent, err
}
func (r *MemberScopedStoreRepository) ReadServiceChargeProof(ctx context.Context, org, op string) (ServiceChargeProof, error) {
	_, proof, err := readServiceCharge(r.db.WithContext(ctx), org, op, false)
	return proof, err
}
func (r *MemberScopedStoreRepository) AdmitServiceCharge(ctx context.Context, intent ServiceChargeIntent) (ServiceChargeIntent, error) {
	if !validServiceIntent(intent) {
		return ServiceChargeIntent{}, ErrServiceQuantityInvalid
	}
	e := intent.Execution
	access, err := r.authorize(ctx, e.OrganizationID, true)
	if err != nil || access.MemberID != intent.MemberID || access.ActorID != e.ActorSubject {
		return ServiceChargeIntent{}, ErrNotFound
	}
	admitted := intent
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockMemberStore(tx, e.OrganizationID, e.StoreID); err != nil {
			return err
		}
		if err := requireMemberGrant(tx, access, e.StoreID); err != nil {
			return err
		}
		_, proof, err := readServiceCharge(tx, e.OrganizationID, e.OperationID, true)
		if err == nil {
			if !sameServiceChargeRequest(proof.Intent, intent) {
				return ErrAlreadyExists
			}
			if proof.Intent.Funding == "enterprise_unallocated" && !access.Administrator && proof.State != "succeeded" {
				return ErrNotFound
			}
			admitted = proof.Intent
			return nil
		}
		if !errors.Is(err, ErrNotFound) {
			return err
		}
		if (intent.Funding == "enterprise_unallocated") != access.Administrator {
			return ErrNotFound
		}
		base, _ := NewGormStoreRepository(tx)
		store, err := base.LockServiceState(ctx, tx, ServiceStoreIdentity{OrganizationID: e.OrganizationID, StoreID: e.StoreID})
		if err != nil {
			return err
		}
		if store.Version != e.ExpectedStoreVersion {
			return ErrVersionConflict
		}
		if store.ConnectionRef != e.ExpectedConnectionRef {
			return ErrConnectionSnapshotChanged
		}
		if _, err := serviceChargeTarget(store.State, e, e.ConnectionStatus, e.OccurredAt); err != nil {
			return err
		}
		encoded, _ := json.Marshal(intent)
		at := time.Now().UTC()
		return tx.Create(&storeServiceChargeRow{OrganizationID: e.OrganizationID, OperationID: e.OperationID, StoreID: e.StoreID, IntentJSON: string(encoded), State: "admitted", CreatedAt: at, UpdatedAt: at}).Error
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAlreadyExists) || errors.Is(err, ErrVersionConflict) || errors.Is(err, ErrConnectionSnapshotChanged) || errors.Is(err, ErrInvalidServiceTransition) || errors.Is(err, ErrConnectionNotFresh) {
			return ServiceChargeIntent{}, err
		}
		if original, readErr := r.ReadServiceChargeIntent(ctx, e.OrganizationID, e.OperationID); readErr == nil && sameServiceChargeRequest(original, intent) {
			return original, nil
		}
		return ServiceChargeIntent{}, err
	}
	return admitted, nil
}
func (r *MemberScopedStoreRepository) BindServiceCharge(ctx context.Context, intent ServiceChargeIntent, reservation string) error {
	if _, err := canonicalUUID(reservation); err != nil {
		return ErrServiceChargeUnknown
	}
	e := intent.Execution
	access, err := r.ServiceMemberAccess(ctx, e.OrganizationID, e.StoreID)
	if err != nil || access.MemberID != intent.MemberID || access.ActorID != e.ActorSubject {
		return ErrNotFound
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockMemberStore(tx, e.OrganizationID, e.StoreID); err != nil {
			return err
		}
		if err := requireMemberGrant(tx, access, e.StoreID); err != nil {
			return err
		}
		row, proof, err := readServiceCharge(tx, e.OrganizationID, e.OperationID, true)
		if err != nil {
			return err
		}
		if !sameServiceChargeIntent(proof.Intent, intent) || row.ReservationID != "" && row.ReservationID != reservation {
			return ErrAlreadyExists
		}
		if row.ReservationID == reservation {
			return nil
		}
		if row.State != "admitted" {
			return ErrServiceChargeUnknown
		}
		return tx.Model(&row).Updates(map[string]any{"reservation_id": reservation, "state": "bound", "updated_at": time.Now().UTC()}).Error
	})
	if err != nil {
		if original, readErr := r.ReadServiceChargeProof(ctx, e.OrganizationID, e.OperationID); readErr == nil && sameServiceChargeIntent(original.Intent, intent) && original.ReservationID == reservation {
			return nil
		}
	}
	return err
}
func serviceChargeTarget(state StoreServiceState, e ServiceExecution, connection ConnectionStatus, at time.Time) (StoreServiceState, error) {
	switch e.Command {
	case ServiceCommandActivate:
		return ActivateStoreService(state, connection, at)
	case ServiceCommandRenew:
		return RenewStoreService(state, e.Quantity, e.MaxQuantity, at)
	case ServiceCommandReactivate:
		return ReactivateStoreService(state, e.Quantity, e.MaxQuantity, at)
	default:
		return StoreServiceState{}, ErrInvalidServiceTransition
	}
}
func completeServiceCharge(tx *gorm.DB, row storeServiceChargeRow, proof ServiceChargeProof) error {
	encoded := ""
	if proof.State == "succeeded" {
		data, _ := json.Marshal(proof.Snapshot)
		encoded = string(data)
	}
	proof.EvidenceID = "store-service:" + hashTuple(row.OrganizationID, row.OperationID, row.ReservationID, proof.State, proof.FailureCode, encoded)
	return tx.Model(&row).Where("state IN ?", []string{"admitted", "bound"}).Updates(map[string]any{"reservation_id": row.ReservationID, "state": proof.State, "snapshot_json": encoded, "evidence_id": proof.EvidenceID, "failure_code": proof.FailureCode, "updated_at": time.Now().UTC()}).Error
}
func (r *MemberScopedStoreRepository) ApplyServiceCharge(ctx context.Context, intent ServiceChargeIntent, connection ConnectionStatus) (ServiceChargeProof, error) {
	e := intent.Execution
	access, accessErr := r.authorize(ctx, e.OrganizationID, true)
	if accessErr != nil && !errors.Is(accessErr, ErrNotFound) {
		return ServiceChargeProof{}, accessErr
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Store first, then assignment, then operation: the same order as revoke.
		storeErr := lockMemberStore(tx, e.OrganizationID, e.StoreID)
		if storeErr != nil && !errors.Is(storeErr, ErrNotFound) {
			return storeErr
		}
		grantErr := accessErr
		if grantErr == nil {
			grantErr = requireMemberGrant(tx, access, e.StoreID)
		}
		if grantErr != nil && !errors.Is(grantErr, ErrNotFound) {
			return grantErr
		}
		row, proof, err := readServiceCharge(tx, e.OrganizationID, e.OperationID, true)
		if err != nil {
			return err
		}
		if !sameServiceChargeIntent(proof.Intent, intent) {
			return ErrAlreadyExists
		}
		if row.State == "succeeded" || row.State == "failed_fenced" {
			return nil
		}
		if row.State != "bound" || row.ReservationID == "" {
			return ErrServiceChargeUnknown
		}
		failure := ""
		if storeErr != nil || grantErr != nil || access.MemberID != intent.MemberID || access.ActorID != e.ActorSubject || intent.Funding == "enterprise_unallocated" && !access.Administrator {
			failure = "NOT_ALLOWED"
		}
		base, _ := NewGormStoreRepository(tx)
		var store ServiceStoreSnapshot
		if failure == "" {
			store, err = base.LockServiceState(ctx, tx, ServiceStoreIdentity{OrganizationID: e.OrganizationID, StoreID: e.StoreID})
			if err != nil {
				return err
			}
			if store.Version != e.ExpectedStoreVersion {
				failure = "STORE_CHANGED"
			} else if store.ConnectionRef != e.ExpectedConnectionRef {
				failure = "CONNECTION_CHANGED"
			}
		}
		at := time.Now().UTC()
		var target StoreServiceState
		if failure == "" {
			target, err = serviceChargeTarget(store.State, e, connection, at)
			if err != nil {
				failure = "INVALID_STATE"
			}
		}
		if failure != "" {
			proof.State = "failed_fenced"
			proof.FailureCode = failure
			return completeServiceCharge(tx, row, proof)
		}
		if err := base.ApplyServiceState(ctx, tx, ServiceStoreMutation{Identity: store.Identity, ExpectedVersion: store.Version, ExpectedConnectionRef: store.ConnectionRef, State: target, ActorSubject: e.ActorSubject, OccurredAt: at}); err != nil {
			return err
		}
		proof.State = "succeeded"
		proof.Snapshot = ServiceOperationSnapshot{OrganizationID: e.OrganizationID, OperationID: e.OperationID, StoreID: e.StoreID, Command: e.Command, Quantity: strconv.FormatInt(e.Quantity, 10), ResourceType: "store_renewal_period", StoreVersion: e.ExpectedStoreVersion + 1, ServiceState: target, EventID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(e.OrganizationID+":"+e.OperationID)).String()}
		return completeServiceCharge(tx, row, proof)
	})
	proof, readErr := r.ReadServiceChargeProof(ctx, e.OrganizationID, e.OperationID)
	if readErr == nil && sameServiceChargeIntent(proof.Intent, intent) && (proof.State == "succeeded" || proof.State == "failed_fenced") {
		return proof, nil
	}
	if err != nil {
		return ServiceChargeProof{}, err
	}
	return ServiceChargeProof{}, ErrServiceChargeUnknown
}

// Only confirmed pre-effect rejection is fenced here. A timeout or database
// read miss cannot call this method to manufacture a no-effect proof.
func (r *MemberScopedStoreRepository) FenceServiceCharge(ctx context.Context, intent ServiceChargeIntent, reservation, code string) (ServiceChargeProof, error) {
	if code != "QUOTA_INSUFFICIENT" && code != "NOT_ALLOWED" && code != "CONNECTION_CHANGED" && code != "INVALID_STATE" {
		return ServiceChargeProof{}, ErrInvalidServiceTransition
	}
	e := intent.Execution
	if reservation != "" {
		if _, err := canonicalUUID(reservation); err != nil {
			return ServiceChargeProof{}, ErrServiceChargeUnknown
		}
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockMemberStore(tx, e.OrganizationID, e.StoreID); err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}
		row, proof, err := readServiceCharge(tx, e.OrganizationID, e.OperationID, true)
		if err != nil {
			return err
		}
		if !sameServiceChargeIntent(proof.Intent, intent) {
			return ErrAlreadyExists
		}
		if row.ReservationID != "" && reservation != "" && row.ReservationID != reservation {
			return ErrAlreadyExists
		}
		if row.State == "succeeded" || row.State == "failed_fenced" {
			return nil
		}
		if row.ReservationID == "" && reservation != "" {
			row.ReservationID = reservation
		}
		proof.State = "failed_fenced"
		proof.FailureCode = code
		return completeServiceCharge(tx, row, proof)
	})
	if err != nil {
		return ServiceChargeProof{}, err
	}
	return r.ReadServiceChargeProof(ctx, e.OrganizationID, e.OperationID)
}
