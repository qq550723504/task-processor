package dataserviceauth

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/dataservice"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/dataacquisition"
	"testing"
	"time"
)

type exactFixture struct {
	grant zitadel.ExactServiceProjectAuthorization
	err   error
}

func (f *exactFixture) ReadExactServiceProjectAuthorization(_ context.Context, token, actor, project, org string) (zitadel.ExactServiceProjectAuthorization, error) {
	if token != "service-only" || actor != "creator" || project != "project" || org != "org" {
		return zitadel.ExactServiceProjectAuthorization{}, errors.New("wrong exact scope")
	}
	return f.grant, f.err
}

type userFixture struct {
	active bool
	err    error
}

func (f *userFixture) IsUserActive(_ context.Context, token, actor string) (bool, error) {
	if token != "service-only" || actor != "creator" {
		return false, errors.New("wrong user")
	}
	return f.active, f.err
}

type statusFixture struct {
	suspended bool
	err       error
}

func (f *statusFixture) IsOrganizationSuspended(context.Context, string) (bool, error) {
	return f.suspended, f.err
}

type policyFixture struct {
	denied string
	admin  bool
	err    error
}

func (f *policyFixture) Authorize(_ string, roles []string, p string) bool {
	return p == "listingkit.platform_admin" && len(roles) == 1 && roles[0] == "platform_admin"
}
func (f *policyFixture) AuthorizeScoped(_ context.Context, actor, org string, roles []string, p string) (bool, error) {
	return actor == "creator" && org == "org" && len(roles) == 1 && roles[0] == "native-current" && p != f.denied, f.err
}

func TestNativeIAMWithoutLocalSuspensionStillRequiresCurrentAuthority(t *testing.T) {
	for _, scenario := range []string{"valid", "missing-grant", "recreated-grant", "inactive-grant", "exact-unavailable", "inactive-user", "user-unavailable", "revoked-module", "policy-unavailable", "token-unavailable", "empty-token", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			exact := &exactFixture{grant: zitadel.ExactServiceProjectAuthorization{Found: true, AuthorizationID: "original", State: "STATE_ACTIVE", Roles: []string{"native-current"}}}
			users := &userFixture{active: true}
			policy := &policyFixture{admin: true}
			token := "service-only"
			var tokenErr error
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			want := dataservice.ErrForbidden
			switch scenario {
			case "missing-grant":
				exact.grant.Found = false
			case "recreated-grant":
				exact.grant.AuthorizationID = "new-member"
			case "inactive-grant":
				exact.grant.State = "STATE_INACTIVE"
			case "exact-unavailable":
				exact.err, want = errors.New("IAM unavailable"), dataservice.ErrUnavailable
			case "inactive-user":
				users.active = false
			case "user-unavailable":
				users.err, want = errors.New("user unavailable"), dataservice.ErrUnavailable
			case "revoked-module":
				policy.denied = dataservice.PermissionManage
			case "policy-unavailable":
				policy.err, want = errors.New("policy unavailable"), dataservice.ErrUnavailable
			case "token-unavailable":
				tokenErr, want = errors.New("credential unavailable"), dataservice.ErrUnavailable
			case "empty-token":
				token, want = "", dataservice.ErrUnavailable
			case "canceled":
				cancel()
				want = dataservice.ErrUnavailable
			}
			a, err := NewAuthorizer(exact, users, func(context.Context) (string, error) { return token, tokenErr }, "project", policy, nil)
			require.NoError(t, err)
			scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}
			err = a.Check(ctx, scope, dataservice.PermissionAcquire)
			if scenario == "valid" {
				require.NoError(t, err)
				require.NoError(t, a.CheckExecution(ctx, dataacquisition.Principal{Scope: scope, CredentialID: "key"}, orgresource.FundingEnterprise))
			} else {
				require.ErrorIs(t, err, want)
			}
		})
	}
}

func TestNativeIAMConstructorRequiresAllFormalDependencies(t *testing.T) {
	exact, users, policy := &exactFixture{}, &userFixture{}, &policyFixture{}
	token := func(context.Context) (string, error) { return "service-only", nil }
	for _, construct := range []func() error{
		func() error { _, e := NewAuthorizer(nil, users, token, "project", policy, nil); return e },
		func() error { _, e := NewAuthorizer(exact, nil, token, "project", policy, nil); return e },
		func() error { _, e := NewAuthorizer(exact, users, nil, "project", policy, nil); return e },
		func() error { _, e := NewAuthorizer(exact, users, token, "", policy, nil); return e },
		func() error { _, e := NewAuthorizer(exact, users, token, "project", nil, nil); return e },
	} {
		require.ErrorIs(t, construct(), dataservice.ErrUnavailable)
	}
}
func (f *policyFixture) IsTenantAdmin(string, []string) bool { return f.admin }
func TestOriginalGrantUserStateModuleAndFrozenFundingAreLive(t *testing.T) {
	exact := &exactFixture{grant: zitadel.ExactServiceProjectAuthorization{Found: true, AuthorizationID: "original", State: "STATE_ACTIVE", Roles: []string{"native-current"}}}
	users := &userFixture{active: true}
	status := &statusFixture{}
	policy := &policyFixture{admin: true}
	a, err := NewAuthorizer(exact, users, func(context.Context) (string, error) { return "service-only", nil }, "project", policy, status)
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org", ActorID: "creator", MemberID: "original"}
	principal := dataacquisition.Principal{Scope: scope, CredentialID: "key"}
	ctx := context.Background()
	require.NoError(t, a.Check(ctx, scope, dataservice.PermissionAcquire))
	require.NoError(t, a.CheckExecution(ctx, principal, orgresource.FundingEnterprise))
	policy.admin = false
	require.ErrorIs(t, a.CheckExecution(ctx, principal, orgresource.FundingEnterprise), dataacquisition.ErrForbidden, "do not switch frozen funding to member allocation")
	require.NoError(t, a.CheckExecution(ctx, principal, orgresource.FundingMember))
	policy.denied = dataservice.PermissionManage
	require.ErrorIs(t, a.Check(ctx, scope, dataservice.PermissionAcquire), dataservice.ErrForbidden)
	require.ErrorIs(t, a.CheckExecution(ctx, principal, orgresource.FundingMember), dataacquisition.ErrForbidden, "API jobs stop when API module is revoked")
	principal.CredentialID = ""
	require.NoError(t, a.CheckExecution(ctx, principal, orgresource.FundingMember), "console market does not require API key management")
	policy.denied = ""
	users.active = false
	require.ErrorIs(t, a.Check(ctx, scope, dataservice.PermissionResult), dataservice.ErrForbidden)
	users.active = true
	exact.grant.AuthorizationID = "recreated"
	require.ErrorIs(t, a.Check(ctx, scope, dataservice.PermissionAcquire), dataservice.ErrForbidden)
	exact.grant.AuthorizationID = "original"
	status.suspended = true
	require.ErrorIs(t, a.Check(ctx, scope, dataservice.PermissionResult), dataservice.ErrForbidden)
	status.suspended = false
	status.err = errors.New("local policy unavailable")
	require.ErrorIs(t, a.Check(ctx, scope, dataservice.PermissionResult), dataservice.ErrUnavailable)
	status.err = nil
	users.err = errors.New("directory unavailable")
	require.ErrorIs(t, a.Check(ctx, scope, dataservice.PermissionResult), dataservice.ErrUnavailable)
	users.err = nil
	identity := authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "creator", EffectiveMemberID: "original", TokenExpiresAt: time.Now().Add(time.Minute), Roles: []string{"stale-role"}}
	resolved, err := a.Resolve(authidentity.WithAuthenticatedIdentity(ctx, identity), dataservice.PermissionManage)
	require.NoError(t, err)
	require.Equal(t, scope, resolved)
	identity.EffectiveOrganizationID = "foreign"
	_, err = a.Resolve(authidentity.WithAuthenticatedIdentity(ctx, identity), dataservice.PermissionManage)
	require.ErrorIs(t, err, dataservice.ErrForbidden)
	identity.TenantID = ""
	identity.EffectiveOrganizationID = ""
	identity.EffectiveMemberID = ""
	identity.Roles = []string{"listingkit_admin"}
	_, err = a.Specialist(authidentity.WithAuthenticatedIdentity(ctx, identity))
	require.ErrorIs(t, err, dataservice.ErrForbidden, "enterprise admin is not a platform specialist")
	identity.Roles = []string{"platform_admin"}
	op, err := a.Specialist(authidentity.WithAuthenticatedIdentity(ctx, identity))
	require.NoError(t, err)
	require.Equal(t, "creator", op.ID)
}
