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

type statusFixture struct{ suspended bool }

func (f *statusFixture) IsOrganizationSuspended(context.Context, string) (bool, error) {
	return f.suspended, nil
}

type policyFixture struct {
	denied string
	admin  bool
}

func (f *policyFixture) Authorize(_ string, roles []string, p string) bool {
	return p == "listingkit.platform_admin" && len(roles) == 1 && roles[0] == "platform_admin"
}
func (f *policyFixture) AuthorizeScoped(_ context.Context, actor, org string, roles []string, p string) (bool, error) {
	return actor == "creator" && org == "org" && len(roles) == 1 && roles[0] == "native-current" && p != f.denied, nil
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
