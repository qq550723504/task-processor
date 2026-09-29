package orgresource

import (
	"context"
	"errors"
	"testing"
)

func TestMemberAllocationChecksCurrentPermissionBeforeRepositoryReplay(t *testing.T) {
	repository := &memberAllocationTestRepository{}
	authorizer := &memberAllocationTestAuthorizer{}
	service, err := NewMemberAllocationService(repository, authorizer)
	if err != nil {
		t.Fatal(err)
	}
	command := MemberResourceTransfer{OrganizationID: "org", MemberID: "membership", ActorID: "admin", OperationID: "allocation", ResourceType: ResourceDataRow, Action: MemberResourceAllocate, Quantity: 1}
	principal := Principal{Kind: PrincipalTenantHuman, ID: "admin"}
	if _, err := service.Transfer(context.Background(), principal, command); err != nil {
		t.Fatal(err)
	}
	authorizer.denied = true
	if _, err := service.Transfer(context.Background(), principal, command); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked administrator replay: %v", err)
	}
	if repository.calls != 1 {
		t.Fatalf("revoked request reached repository %d times", repository.calls)
	}
	command.ActorID = "different-user"
	if _, err := service.Transfer(context.Background(), principal, command); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("wrong actor accepted: %v", err)
	}
	if _, err := service.ReadPosition(context.Background(), principal, "different-org", "membership", ResourceDataRow); !errors.Is(err, ErrForbidden) {
		t.Fatalf("cross-org read reached repository: %v", err)
	}
	if repository.calls != 1 {
		t.Fatal("forbidden read reached repository")
	}
}

type memberAllocationTestRepository struct{ calls int }

func (r *memberAllocationTestRepository) Transfer(context.Context, MemberResourceTransfer) (MemberResourceTransferResult, error) {
	r.calls++
	return MemberResourceTransferResult{}, nil
}
func (r *memberAllocationTestRepository) ReadPosition(context.Context, string, string, ResourceType) (MemberResourcePosition, error) {
	r.calls++
	return MemberResourcePosition{}, nil
}

type memberAllocationTestAuthorizer struct{ denied bool }

func (a *memberAllocationTestAuthorizer) AuthorizeMemberResourceTransfer(context.Context, Principal, MemberResourceTransfer) error {
	if a.denied {
		return ErrForbidden
	}
	return nil
}
func (a *memberAllocationTestAuthorizer) AuthorizeMemberResourceRead(context.Context, Principal, string, string) error {
	if a.denied {
		return ErrForbidden
	}
	return nil
}
