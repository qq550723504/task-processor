package orgresource

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type ResourceConsumer string

const (
	ConsumerStoreService       ResourceConsumer = "store_service_v1"
	ConsumerProductAcquisition ResourceConsumer = "product_acquisition_v1"
)

type ResourceFunding string

const (
	FundingEnterprise ResourceFunding = "enterprise_unallocated"
	FundingMember     ResourceFunding = "member_allocated"
)

type ConsumerEffectState string

const (
	ConsumerEffectUnknown   ConsumerEffectState = "unknown"
	ConsumerEffectSucceeded ConsumerEffectState = "succeeded"
	ConsumerEffectFailed    ConsumerEffectState = "failed_fenced"
)

var ErrConsumerChargeUnknown = errors.New("consumer effect is not yet proven terminal")

type ConsumerChargeIdentity struct {
	OrganizationID string
	Consumer       ResourceConsumer
	OperationID    string
}

// This intent must already be immutable and admitted in the consumer's own DB.
// The charge port reads it from a registered owner, never from a public caller.
type ConsumerChargeIntent struct {
	Identity      ConsumerChargeIdentity
	ActorID       string
	MemberID      string
	Funding       ResourceFunding
	ResourceType  ResourceType
	Quantity      int64
	Fingerprint   string
	BusinessScope string
}
type ConsumerChargeReceipt struct {
	Intent          ConsumerChargeIntent
	ReservationID   string
	State           ReservationState
	OwnerEvidenceID string
	CreatedAt       time.Time
	BalanceAfter    int64
}
type ConsumerChargeProof struct {
	Intent        ConsumerChargeIntent
	ReservationID string
	State         ConsumerEffectState
	EvidenceID    string
}
type ConsumerChargeOwner interface {
	ReadChargeIntent(context.Context, ConsumerChargeIdentity) (ConsumerChargeIntent, error)
	ReadChargeProof(context.Context, ConsumerChargeReceipt) (ConsumerChargeProof, error)
}
type ConsumerChargeRepository interface {
	Reserve(context.Context, ConsumerChargeIntent) (ConsumerChargeReceipt, error)
	Read(context.Context, ConsumerChargeIdentity) (ConsumerChargeReceipt, error)
	Settle(context.Context, ConsumerChargeReceipt, ConsumerChargeProof) (ConsumerChargeReceipt, error)
	ClaimDue(context.Context, []ResourceConsumer) ([]ConsumerChargeIdentity, error)
}

// Owners use this narrow in-process port; it has no caller-selected principal,
// proof, settlement decision or resource quantity.
type ConsumerChargePort interface {
	Reserve(context.Context, ConsumerChargeIdentity) (ConsumerChargeReceipt, error)
	Reconcile(context.Context, ConsumerChargeIdentity) (ConsumerChargeReceipt, error)
}
type ConsumerChargeService struct {
	repository ConsumerChargeRepository
	owners     map[ResourceConsumer]ConsumerChargeOwner
}

func NewConsumerChargeService(repository ConsumerChargeRepository, owners map[ResourceConsumer]ConsumerChargeOwner) (*ConsumerChargeService, error) {
	if repository == nil || len(owners) == 0 {
		return nil, ErrInvalidInput
	}
	registered := make(map[ResourceConsumer]ConsumerChargeOwner, len(owners))
	for consumer, owner := range owners {
		if !validResourceConsumer(consumer) || owner == nil {
			return nil, ErrReservationOwnerNotRegistered
		}
		registered[consumer] = owner
	}
	return &ConsumerChargeService{repository: repository, owners: registered}, nil
}
func (s *ConsumerChargeService) Reserve(ctx context.Context, identity ConsumerChargeIdentity) (ConsumerChargeReceipt, error) {
	if !ValidConsumerChargeIdentity(identity) {
		return ConsumerChargeReceipt{}, ErrInvalidInput
	}
	owner, ok := s.owners[identity.Consumer]
	if !ok {
		return ConsumerChargeReceipt{}, ErrReservationOwnerNotRegistered
	}
	readContext, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	intent, err := owner.ReadChargeIntent(readContext, identity)
	cancel()
	if err != nil {
		return ConsumerChargeReceipt{}, err
	}
	if intent.Identity != identity || !ValidConsumerChargeIntent(intent) {
		return ConsumerChargeReceipt{}, ErrOwnerScopeMismatch
	}
	return s.repository.Reserve(ctx, intent)
}
func (s *ConsumerChargeService) Reconcile(ctx context.Context, identity ConsumerChargeIdentity) (ConsumerChargeReceipt, error) {
	if !ValidConsumerChargeIdentity(identity) {
		return ConsumerChargeReceipt{}, ErrInvalidInput
	}
	owner, ok := s.owners[identity.Consumer]
	if !ok {
		return ConsumerChargeReceipt{}, ErrReservationOwnerNotRegistered
	}
	receipt, err := s.repository.Read(ctx, identity)
	if err != nil {
		return ConsumerChargeReceipt{}, err
	}
	if receipt.Intent.Identity != identity || !ValidConsumerChargeIntent(receipt.Intent) {
		return ConsumerChargeReceipt{}, ErrOwnerScopeMismatch
	}
	if receipt.State == ReservationCommitted || receipt.State == ReservationReleased {
		return receipt, nil
	}
	readContext, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	proof, err := owner.ReadChargeProof(readContext, receipt)
	cancel()
	if err != nil {
		return receipt, err
	}
	if proof.Intent != receipt.Intent || proof.ReservationID != receipt.ReservationID {
		return receipt, ErrIdempotencyKeyConflict
	}
	if proof.State == ConsumerEffectUnknown {
		return receipt, ErrConsumerChargeUnknown
	}
	if (proof.State != ConsumerEffectSucceeded && proof.State != ConsumerEffectFailed) || !validChargeEvidence(proof.EvidenceID) {
		return receipt, ErrConsumerChargeUnknown
	}
	return s.repository.Settle(ctx, receipt, proof)
}
func (s *ConsumerChargeService) RecoverDue(ctx context.Context) (int, error) {
	consumers := make([]ResourceConsumer, 0, len(s.owners))
	for consumer := range s.owners {
		consumers = append(consumers, consumer)
	}
	identities, err := s.repository.ClaimDue(ctx, consumers)
	if err != nil {
		return 0, err
	}
	settled := 0
	var failures []error
	for _, identity := range identities {
		if err := ctx.Err(); err != nil {
			return settled, errors.Join(append(failures, err)...)
		}
		receipt, err := s.Reconcile(ctx, identity)
		if err == nil && (receipt.State == ReservationCommitted || receipt.State == ReservationReleased) {
			settled++
		} else if err != nil && !errors.Is(err, ErrConsumerChargeUnknown) {
			failures = append(failures, err)
		}
	}
	return settled, errors.Join(failures...)
}
func ValidConsumerChargeIntent(intent ConsumerChargeIntent) bool {
	_, fingerprintError := hex.DecodeString(intent.Fingerprint)
	if !ValidConsumerChargeIdentity(intent.Identity) || !validLimitIdentity(intent.ActorID) || !validLimitIdentity(intent.MemberID) || len(intent.Fingerprint) != 64 || fingerprintError != nil || strings.ToLower(intent.Fingerprint) != intent.Fingerprint || !validChargeEvidence(intent.BusinessScope) || (intent.Funding != FundingEnterprise && intent.Funding != FundingMember) {
		return false
	}
	switch intent.Identity.Consumer {
	case ConsumerProductAcquisition:
		return intent.ResourceType == ResourceDataRow && intent.Quantity == 1
	case ConsumerStoreService:
		return intent.ResourceType == ResourceStoreRenewalPeriod && intent.Quantity > 0
	default:
		return false
	}
}
func validResourceConsumer(consumer ResourceConsumer) bool {
	return consumer == ConsumerProductAcquisition || consumer == ConsumerStoreService
}
func ValidConsumerChargeIdentity(identity ConsumerChargeIdentity) bool {
	return validLimitIdentity(identity.OrganizationID) && validLimitIdentity(identity.OperationID) && validResourceConsumer(identity.Consumer)
}
func validChargeEvidence(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && len(value) <= 256 && !strings.ContainsAny(value, "\x00\r\n")
}
