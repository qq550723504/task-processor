package orgresource

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrMemberLimitUnavailable     = errors.New("member AI point monthly limit is unavailable")
	ErrMemberLimitExceeded        = errors.New("member AI point monthly limit is insufficient")
	ErrMemberLimitVersionConflict = errors.New("member AI point limit version conflict")
)

// A limit constrains use of the enterprise bucket; it is not another balance.
type MemberLimitSnapshot struct {
	OrganizationID string
	MemberID       string
	MonthlyLimit   int64
	Version        int64
	MonthStart     time.Time
	Reserved       int64
	Consumed       int64
}
type SetMemberLimitExecution struct {
	OrganizationID  string
	MemberID        string
	OperationID     string
	ActorID         string
	ExpectedVersion int64
	Target          int64
}
type MemberLimitRepository interface {
	SetMonthlyLimit(context.Context, SetMemberLimitExecution) (MemberLimitSnapshot, error)
	ReadMonthlyLimit(context.Context, string, string) (MemberLimitSnapshot, error)
}

// Runtime assembly must check the current organization, live administrator
// permission and exact active canonical target member. Token allocation is not
// evidence of AI point permission.
type MemberLimitAuthorizer interface {
	AuthorizeMemberLimitRead(context.Context, Principal, string, string) error
	AuthorizeMemberLimitWrite(context.Context, Principal, string, string) error
}
type MemberLimitService struct {
	repository MemberLimitRepository
	authorizer MemberLimitAuthorizer
}

func NewMemberLimitService(repository MemberLimitRepository, authorizer MemberLimitAuthorizer) (*MemberLimitService, error) {
	if repository == nil || authorizer == nil {
		return nil, ErrInvalidInput
	}
	return &MemberLimitService{repository: repository, authorizer: authorizer}, nil
}
func (s *MemberLimitService) SetMonthlyLimit(ctx context.Context, principal Principal, input SetMemberLimitExecution) (MemberLimitSnapshot, error) {
	if input.ActorID != principal.ID || principal.Kind != PrincipalTenantHuman || !ValidMemberLimitCommand(input) {
		return MemberLimitSnapshot{}, ErrInvalidInput
	}
	if err := s.authorizer.AuthorizeMemberLimitWrite(ctx, principal, input.OrganizationID, input.MemberID); err != nil {
		return MemberLimitSnapshot{}, err
	}
	return s.repository.SetMonthlyLimit(ctx, input)
}
func (s *MemberLimitService) ReadMonthlyLimit(ctx context.Context, principal Principal, org, member string) (MemberLimitSnapshot, error) {
	if !validLimitIdentity(org) || !validLimitIdentity(member) || principal.Kind != PrincipalTenantHuman {
		return MemberLimitSnapshot{}, ErrInvalidInput
	}
	if err := s.authorizer.AuthorizeMemberLimitRead(ctx, principal, org, member); err != nil {
		return MemberLimitSnapshot{}, err
	}
	return s.repository.ReadMonthlyLimit(ctx, org, member)
}
func ValidMemberLimitCommand(input SetMemberLimitExecution) bool {
	return validLimitIdentity(input.OrganizationID) && validLimitIdentity(input.MemberID) && validLimitIdentity(input.OperationID) && validLimitIdentity(input.ActorID) && input.ExpectedVersion >= 0 && input.Target >= 0
}
func validLimitIdentity(s string) bool { return s != "" && strings.TrimSpace(s) == s && len(s) <= 128 }
func AIPointMonthStart(now time.Time) time.Time {
	v := now.UTC()
	return time.Date(v.Year(), v.Month(), 1, 0, 0, 0, 0, time.UTC)
}
