package toolmarketauth

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	tm "task-processor/internal/toolmarket"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

type resolver struct{ roles []string }

func (r resolver) Resolve(_ context.Context, p httproute.OrganizationAccessPolicy, in workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	if p != httproute.OrganizationAccessPolicyLiveWrite {
		panic("not fresh")
	}
	id := in.Identity
	id.TenantID = in.RequestedOrganizationID
	id.EffectiveOrganizationID = id.TenantID
	id.EffectiveMemberID = "member"
	id.Roles = r.roles
	return id, nil
}

type policies struct{}

func (policies) Authorize(_ string, roles []string, p string) bool {
	for _, r := range roles {
		if r == "platform_admin" && p == authz.PermissionListingKitPlatformAdm {
			return true
		}
	}
	return false
}
func (policies) AuthorizeScoped(context.Context, string, string, []string, string) (bool, error) {
	return true, nil
}
func TestManageCannotBeGrantedByCustomModule(t *testing.T) {
	a, e := New(resolver{[]string{"sumi_role_custom"}}, policies{})
	require.NoError(t, e)
	id := authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)}
	ctx, e := a.Bind(authidentity.WithAuthenticatedIdentity(context.Background(), id), "Bearer verified-token")
	require.NoError(t, e)
	_, e = a.Authorize(ctx, authz.PermissionWorkbenchToolsRead, false)
	require.NoError(t, e)
	_, e = a.Authorize(ctx, authz.PermissionWorkbenchToolsManage, false)
	require.ErrorIs(t, e, tm.ErrForbidden)
	a, e = New(resolver{[]string{"listingkit_admin"}}, policies{})
	require.NoError(t, e)
	_, e = a.Authorize(ctx, authz.PermissionWorkbenchToolsManage, false)
	require.NoError(t, e)
}
func TestPlatformUsesVerifiedRolesWithoutCustomerGrant(t *testing.T) {
	a, e := New(nil, policies{})
	require.Error(t, e)
	a, e = New(resolver{}, policies{})
	require.NoError(t, e)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "specialist", Roles: []string{"platform_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)})
	scope, e := a.Authorize(ctx, authz.PermissionListingKitPlatformAdm, true)
	require.NoError(t, e)
	require.Equal(t, tm.Scope{ActorID: "specialist"}, scope)
	_, e = a.Authorize(ctx, authz.PermissionWorkbenchToolsManage, true)
	require.ErrorIs(t, e, tm.ErrForbidden)
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "operator", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)})
	_, e = a.Authorize(ctx, authz.PermissionListingKitPlatformAdm, true)
	require.ErrorIs(t, e, tm.ErrForbidden)
}
