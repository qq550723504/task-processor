package storecenterapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
)

type ServiceChargeRepository interface {
	storecenter.ServiceChargeStore
	Get(context.Context, string, string) (*storecenter.Store, error)
	ServiceMemberAccess(context.Context, string, string) (storecenter.StoreMemberAccess, error)
}

// The application coordinates two owners through original intents and receipts.
// Both owners commit their own transaction; no shared-DB executor is used.
type ServiceLifecycleExecutor struct {
	stores      ServiceChargeRepository
	charges     orgresource.ConsumerChargePort
	connections storecenter.ConnectionStatusProvider
}

func NewServiceLifecycleExecutor(stores ServiceChargeRepository, charges orgresource.ConsumerChargePort, connections storecenter.ConnectionStatusProvider) (*ServiceLifecycleExecutor, error) {
	if stores == nil || charges == nil || connections == nil {
		return nil, storecenter.ErrDependencyUnavailable
	}
	return &ServiceLifecycleExecutor{stores: stores, charges: charges, connections: connections}, nil
}

var _ storecenter.ServiceLifecycleExecutor = (*ServiceLifecycleExecutor)(nil)

func (s *ServiceLifecycleExecutor) authorizeIntent(ctx context.Context, intent storecenter.ServiceChargeIntent) error {
	e := intent.Execution
	access, err := s.stores.ServiceMemberAccess(ctx, e.OrganizationID, e.StoreID)
	if err != nil {
		return err
	}
	if access.MemberID != intent.MemberID || access.ActorID != e.ActorSubject {
		return storecenter.ErrNotFound
	}
	return nil
}
func (s *ServiceLifecycleExecutor) ReplayServiceLifecycle(ctx context.Context, replay storecenter.ServiceReplay) (storecenter.ServiceOperationResult, bool, error) {
	intent, err := s.stores.ReadServiceChargeIntent(ctx, replay.OrganizationID, replay.OperationID)
	if errors.Is(err, storecenter.ErrNotFound) {
		return storecenter.ServiceOperationResult{}, false, nil
	}
	if err != nil {
		return storecenter.ServiceOperationResult{}, false, err
	}
	if err := s.authorizeIntent(ctx, intent); err != nil {
		return storecenter.ServiceOperationResult{}, true, err
	}
	if intent.Execution.RequestFingerprint != replay.RequestFingerprint {
		return storecenter.ServiceOperationResult{}, true, orgresource.ErrIdempotencyKeyConflict
	}
	result, err := s.resume(ctx, intent)
	result.Replayed = true
	return result, true, err
}
func (s *ServiceLifecycleExecutor) ExecuteServiceLifecycle(ctx context.Context, execution storecenter.ServiceExecution) (storecenter.ServiceOperationResult, error) {
	access, err := s.stores.ServiceMemberAccess(ctx, execution.OrganizationID, execution.StoreID)
	if err != nil {
		return storecenter.ServiceOperationResult{}, err
	}
	funding := "member_allocated"
	if access.Administrator {
		funding = "enterprise_unallocated"
	}
	intent, err := s.stores.AdmitServiceCharge(ctx, storecenter.ServiceChargeIntent{Execution: execution, MemberID: access.MemberID, Funding: funding})
	if errors.Is(err, storecenter.ErrAlreadyExists) {
		err = orgresource.ErrIdempotencyKeyConflict
	}
	if err != nil {
		return storecenter.ServiceOperationResult{}, err
	}
	return s.resume(ctx, intent)
}
func (s *ServiceLifecycleExecutor) resume(ctx context.Context, intent storecenter.ServiceChargeIntent) (storecenter.ServiceOperationResult, error) {
	e := intent.Execution
	proof, err := s.stores.ReadServiceChargeProof(ctx, e.OrganizationID, e.OperationID)
	if err != nil {
		return storecenter.ServiceOperationResult{}, err
	}
	if proof.State == "succeeded" || proof.State == "failed_fenced" {
		return s.settle(ctx, proof)
	}
	access, err := s.stores.ServiceMemberAccess(ctx, e.OrganizationID, e.StoreID)
	if err != nil {
		return storecenter.ServiceOperationResult{}, err
	}
	if intent.Funding == "enterprise_unallocated" && !access.Administrator {
		return storecenter.ServiceOperationResult{}, storecenter.ErrNotFound
	}
	receipt, err := s.charges.Reserve(ctx, serviceChargeIdentity(intent))
	if errors.Is(err, orgresource.ErrInsufficientBalance) || errors.Is(err, orgresource.ErrResourceDebtOutstanding) {
		if _, fenceErr := s.stores.FenceServiceCharge(ctx, intent, "", "QUOTA_INSUFFICIENT"); fenceErr != nil {
			return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
		}
		return storecenter.ServiceOperationResult{}, orgresource.ErrInsufficientBalance
	}
	if err != nil {
		return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
	}
	if receipt.Intent != serviceResourceIntent(intent) || receipt.State != orgresource.ReservationReserved {
		return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
	}
	if err := s.stores.BindServiceCharge(ctx, intent, receipt.ReservationID); err != nil {
		if errors.Is(err, storecenter.ErrNotFound) && ctx.Err() == nil {
			if proof, fenceErr := s.stores.FenceServiceCharge(ctx, intent, receipt.ReservationID, "NOT_ALLOWED"); fenceErr == nil {
				return s.settle(ctx, proof)
			}
		}
		return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
	}
	connection := storecenter.ConnectionStatus("")
	if e.Command == storecenter.ServiceCommandActivate {
		store, err := s.stores.Get(ctx, e.OrganizationID, e.StoreID)
		if err != nil {
			return storecenter.ServiceOperationResult{}, err
		}
		if store.ConnectionRef() != e.ExpectedConnectionRef {
			proof, err := s.stores.FenceServiceCharge(ctx, intent, receipt.ReservationID, "CONNECTION_CHANGED")
			if err != nil {
				return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
			}
			return s.settle(ctx, proof)
		}
		call, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		connection, err = s.connections.Status(call, storecenter.ConnectionStatusInput{OrganizationID: e.OrganizationID, StoreID: e.StoreID, Platform: store.Platform(), ConnectionRef: store.ConnectionRef()})
		cancel()
		if err != nil || connection == storecenter.ConnectionStatusUnavailable || connection == "" {
			return storecenter.ServiceOperationResult{}, storecenter.ErrConnectionUnavailable
		}
	}
	proof, err = s.stores.ApplyServiceCharge(ctx, intent, connection)
	if err != nil {
		return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
	}
	return s.settle(ctx, proof)
}
func (s *ServiceLifecycleExecutor) settle(ctx context.Context, proof storecenter.ServiceChargeProof) (storecenter.ServiceOperationResult, error) {
	if proof.State == "failed_fenced" && proof.ReservationID == "" {
		return storecenter.ServiceOperationResult{}, serviceFailure(proof.FailureCode)
	}
	receipt, err := s.charges.Reconcile(ctx, serviceChargeIdentity(proof.Intent))
	if err != nil || receipt.Intent != serviceResourceIntent(proof.Intent) || receipt.ReservationID != proof.ReservationID {
		return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
	}
	if proof.State == "failed_fenced" {
		if receipt.State != orgresource.ReservationReleased {
			return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
		}
		return storecenter.ServiceOperationResult{}, serviceFailure(proof.FailureCode)
	}
	if proof.State != "succeeded" || receipt.State != orgresource.ReservationCommitted {
		return storecenter.ServiceOperationResult{}, storecenter.ErrServiceChargeUnknown
	}
	snapshot := proof.Snapshot
	snapshot.BalanceAfter = strconv.FormatInt(receipt.BalanceAfter, 10)
	return storecenter.ServiceOperationResult{Snapshot: snapshot}, nil
}
func serviceFailure(code string) error {
	switch code {
	case "NOT_ALLOWED":
		return storecenter.ErrNotFound
	case "STORE_CHANGED":
		return storecenter.ErrVersionConflict
	case "CONNECTION_CHANGED":
		return storecenter.ErrConnectionSnapshotChanged
	case "QUOTA_INSUFFICIENT":
		return orgresource.ErrInsufficientBalance
	default:
		return storecenter.ErrInvalidServiceTransition
	}
}

// Registered server-owned proof reads always use the original member/funding
// binding, including settlement after membership or assignment revocation.
type ServiceChargeOwner struct {
	stores storecenter.ServiceChargeStore
}

func NewServiceChargeOwner(stores storecenter.ServiceChargeStore) (*ServiceChargeOwner, error) {
	if stores == nil {
		return nil, storecenter.ErrDependencyUnavailable
	}
	return &ServiceChargeOwner{stores: stores}, nil
}
func (o *ServiceChargeOwner) ReadChargeIntent(ctx context.Context, identity orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeIntent, error) {
	if identity.Consumer != orgresource.ConsumerStoreService {
		return orgresource.ConsumerChargeIntent{}, orgresource.ErrOwnerScopeMismatch
	}
	intent, err := o.stores.ReadServiceChargeIntent(ctx, identity.OrganizationID, identity.OperationID)
	if err != nil {
		return orgresource.ConsumerChargeIntent{}, err
	}
	return serviceResourceIntent(intent), nil
}
func (o *ServiceChargeOwner) ReadChargeProof(ctx context.Context, receipt orgresource.ConsumerChargeReceipt) (orgresource.ConsumerChargeProof, error) {
	identity := receipt.Intent.Identity
	if identity.Consumer != orgresource.ConsumerStoreService {
		return orgresource.ConsumerChargeProof{}, orgresource.ErrOwnerScopeMismatch
	}
	proof, err := o.stores.ReadServiceChargeProof(ctx, identity.OrganizationID, identity.OperationID)
	if err != nil {
		return orgresource.ConsumerChargeProof{}, err
	}
	state := orgresource.ConsumerEffectUnknown
	reservation := proof.ReservationID
	if reservation == "" {
		reservation = receipt.ReservationID
	}
	if proof.ReservationID != "" {
		switch proof.State {
		case "succeeded":
			state = orgresource.ConsumerEffectSucceeded
		case "failed_fenced":
			state = orgresource.ConsumerEffectFailed
		}
	}
	return orgresource.ConsumerChargeProof{Intent: serviceResourceIntent(proof.Intent), ReservationID: reservation, State: state, EvidenceID: proof.EvidenceID}, nil
}
func serviceChargeIdentity(intent storecenter.ServiceChargeIntent) orgresource.ConsumerChargeIdentity {
	return orgresource.ConsumerChargeIdentity{OrganizationID: intent.Execution.OrganizationID, Consumer: orgresource.ConsumerStoreService, OperationID: intent.Execution.OperationID}
}
func serviceResourceIntent(intent storecenter.ServiceChargeIntent) orgresource.ConsumerChargeIntent {
	data, _ := json.Marshal(intent)
	hash := sha256.Sum256(data)
	e := intent.Execution
	return orgresource.ConsumerChargeIntent{Identity: serviceChargeIdentity(intent), ActorID: e.ActorSubject, MemberID: intent.MemberID, Funding: orgresource.ResourceFunding(intent.Funding), ResourceType: orgresource.ResourceStoreRenewalPeriod, Quantity: e.Quantity, Fingerprint: hex.EncodeToString(hash[:]), BusinessScope: e.StoreID}
}
