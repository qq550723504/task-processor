package storeobservationsapp

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	o "task-processor/internal/marketplace/shein/observations"
	"testing"
	"time"
)

type fakeIAM struct {
	result zitadel.ExactServiceProjectAuthorization
	err    error
	calls  int
}

func (f *fakeIAM) ReadExactServiceProjectAuthorization(context.Context, string, string, string, string) (zitadel.ExactServiceProjectAuthorization, error) {
	f.calls++
	return f.result, f.err
}
func TestOriginalMembershipLiveAuthorizationAndOutage(t *testing.T) {
	policy, e := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, e)
	iam := &fakeIAM{result: zitadel.ExactServiceProjectAuthorization{Found: true, State: "STATE_ACTIVE", AuthorizationID: "member-a", Roles: []string{"role-a"}}}
	a := Authorization{Client: iam, Permissions: policy, ProjectID: "project-a", ServiceToken: func(context.Context) (string, error) { return "private", nil }}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	scope := o.Scope{"org-a", "actor-a", "member-a"}
	roles, e := a.current(ctx, scope)
	require.NoError(t, e)
	require.Equal(t, []string{"role-a"}, roles)
	replacement := scope
	replacement.MemberID = "new-member"
	_, e = a.current(ctx, replacement)
	require.ErrorIs(t, e, o.ErrForbidden)
	iam.err = errors.New("temporary")
	_, e = a.current(ctx, scope)
	require.ErrorIs(t, e, o.ErrUnavailable)
	request := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: "org-b", EffectiveOrganizationID: "org-b", UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	calls := iam.calls
	_, e = a.current(request, scope)
	require.ErrorIs(t, e, o.ErrForbidden)
	require.Equal(t, calls, iam.calls)
}
