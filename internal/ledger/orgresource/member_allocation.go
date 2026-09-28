package orgresource

import (
	"context"
	"errors"
	"time"
)

var (
	ErrMemberResourceVersionConflict = errors.New("member resource position version conflict")
	ErrResourceDebtOutstanding       = errors.New("organization resource debt must be repaid before spending")
)

type MemberResourceAction string

const (
	MemberResourceAllocate MemberResourceAction = "allocate_member_resource"
	MemberResourceReclaim  MemberResourceAction = "reclaim_member_resource"
)

// MemberID is the canonical membership authorization ID, not the user's ID.
type MemberResourceTransfer struct {
	OrganizationID  string
	MemberID        string
	ActorID         string
	OperationID     string
	ResourceType    ResourceType
	Action          MemberResourceAction
	Quantity        int64
	ExpectedVersion int64
}
type MemberResourcePosition struct {
	OrganizationID string
	MemberID       string
	ResourceType   ResourceType
	Free           int64
	Reserved       int64
	Consumed       int64
	Version        int64
	UpdatedAt      time.Time
}
type MemberResourceTransferResult struct {
	Position    MemberResourcePosition
	Unallocated int64
	Allocated   int64
	GrossCredit int64
	DebtRepaid  int64
	NetCredit   int64
	Replayed    bool
}
type MemberAllocationRepository interface {
	Transfer(context.Context, MemberResourceTransfer) (MemberResourceTransferResult, error)
	ReadPosition(context.Context, string, string, ResourceType) (MemberResourcePosition, error)
}

// Allocation requires a live active target. Reclaim also permits a departed
// target's durable position, but always requires current enterprise admin rights.
type MemberAllocationAuthorizer interface {
	AuthorizeMemberResourceTransfer(context.Context, Principal, MemberResourceTransfer) error
	AuthorizeMemberResourceRead(context.Context, Principal, string, string) error
}
type MemberAllocationService struct {
	repository MemberAllocationRepository
	authorizer MemberAllocationAuthorizer
}

func NewMemberAllocationService(repository MemberAllocationRepository, authorizer MemberAllocationAuthorizer) (*MemberAllocationService, error) {
	if repository == nil || authorizer == nil {
		return nil, ErrInvalidInput
	}
	return &MemberAllocationService{repository: repository, authorizer: authorizer}, nil
}
func (s *MemberAllocationService) Transfer(ctx context.Context, principal Principal, command MemberResourceTransfer) (MemberResourceTransferResult, error) {
	if principal.Kind != PrincipalTenantHuman || principal.ID != command.ActorID || !ValidMemberResourceTransfer(command) {
		return MemberResourceTransferResult{}, ErrInvalidInput
	}
	if err := s.authorizer.AuthorizeMemberResourceTransfer(ctx, principal, command); err != nil {
		return MemberResourceTransferResult{}, err
	}
	return s.repository.Transfer(ctx, command)
}
func (s *MemberAllocationService) ReadPosition(ctx context.Context, principal Principal, org, member string, resource ResourceType) (MemberResourcePosition, error) {
	if principal.Kind != PrincipalTenantHuman || !validLimitIdentity(principal.ID) || !ValidMemberResourcePositionIdentity(org, member, resource) {
		return MemberResourcePosition{}, ErrInvalidInput
	}
	if err := s.authorizer.AuthorizeMemberResourceRead(ctx, principal, org, member); err != nil {
		return MemberResourcePosition{}, err
	}
	return s.repository.ReadPosition(ctx, org, member, resource)
}
func IsMemberAllocatedResource(resource ResourceType) bool {
	return resource == ResourceStoreRenewalPeriod || resource == ResourceDataRow
}
func ValidMemberResourcePositionIdentity(org, member string, resource ResourceType) bool {
	return validLimitIdentity(org) && validLimitIdentity(member) && IsMemberAllocatedResource(resource)
}
func ValidMemberResourceTransfer(c MemberResourceTransfer) bool {
	return validLimitIdentity(c.OrganizationID) && validLimitIdentity(c.MemberID) && validLimitIdentity(c.ActorID) && validLimitIdentity(c.OperationID) && IsMemberAllocatedResource(c.ResourceType) && (c.Action == MemberResourceAllocate || c.Action == MemberResourceReclaim) && c.Quantity > 0 && c.ExpectedVersion >= 0
}
