package orgresource

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memberLimitTestRepository struct{ calls int }

func (r *memberLimitTestRepository) SetMonthlyLimit(context.Context, SetMemberLimitExecution) (MemberLimitSnapshot, error) {
	r.calls++
	return MemberLimitSnapshot{}, nil
}
func (r *memberLimitTestRepository) ReadMonthlyLimit(context.Context, string, string) (MemberLimitSnapshot, error) {
	r.calls++
	return MemberLimitSnapshot{}, nil
}

type memberLimitTestAuthorization struct{ err error }

func (a *memberLimitTestAuthorization) AuthorizeMemberLimitRead(context.Context, Principal, string, string) error {
	return a.err
}
func (a *memberLimitTestAuthorization) AuthorizeMemberLimitWrite(context.Context, Principal, string, string) error {
	return a.err
}
func TestMemberLimitServiceRequiresLiveAuthorizationEvenForReplay(t *testing.T) {
	r := &memberLimitTestRepository{}
	auth := &memberLimitTestAuthorization{err: ErrForbidden}
	s, err := NewMemberLimitService(r, auth)
	require.NoError(t, err)
	p := Principal{ID: "admin-1", Kind: PrincipalTenantHuman}
	input := SetMemberLimitExecution{OrganizationID: "org-1", MemberID: "member-1", OperationID: "op-1", ActorID: p.ID, Target: 0}
	_, err = s.SetMonthlyLimit(context.Background(), p, input)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = s.ReadMonthlyLimit(context.Background(), p, "org-1", "member-1")
	require.ErrorIs(t, err, ErrForbidden)
	require.Zero(t, r.calls)
	auth.err = nil
	input.ActorID = "other-admin"
	_, err = s.SetMonthlyLimit(context.Background(), p, input)
	require.ErrorIs(t, err, ErrInvalidInput)
	require.Zero(t, r.calls)
	local := time.Date(2026, 10, 1, 0, 30, 0, 0, time.FixedZone("UTC+8", 8*3600))
	require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), AIPointMonthStart(local))
}
