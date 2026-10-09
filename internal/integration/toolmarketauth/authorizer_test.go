package toolmarketauth

import (
	"context"
	"errors"
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

type policyResolver struct {
	policies []httproute.OrganizationAccessPolicy
	liveErr  error
	mutate   func(*authidentity.AuthenticatedIdentity)
}

func (r *policyResolver) Resolve(_ context.Context, p httproute.OrganizationAccessPolicy, in workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	r.policies = append(r.policies, p)
	if p == httproute.OrganizationAccessPolicyLiveWrite && r.liveErr != nil {
		return authidentity.AuthenticatedIdentity{}, r.liveErr
	}
	id := in.Identity
	id.TenantID = in.RequestedOrganizationID
	id.EffectiveOrganizationID = in.RequestedOrganizationID
	id.EffectiveMemberID = "member"
	id.Roles = []string{"listingkit_admin"}
	if r.mutate != nil {
		r.mutate(&id)
	}
	return id, nil
}

func TestCachedReadSurvivesLiveDirectoryOutageWhileWritesFailClosed(t *testing.T) {
	r := &policyResolver{liveErr: errors.New("live directory unavailable")}
	a, err := New(r, policies{})
	require.NoError(t, err)
	id := authidentity.AuthenticatedIdentity{UserID: "actor", TenantID: "org", EffectiveOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)}
	ctx, err := a.Bind(authidentity.WithAuthenticatedIdentity(context.Background(), id), "Bearer verified-token")
	require.NoError(t, err)
	for _, permission := range []string{authz.PermissionWorkbenchToolsRead, authz.PermissionWorkbenchToolsManage, authz.PermissionWorkbenchToolsCustomize} {
		scope, err := a.AuthorizeRead(ctx, permission, false)
		require.NoError(t, err)
		require.Equal(t, tm.Scope{ActorID: "actor", OrganizationID: "org"}, scope)
	}
	require.Equal(t, []httproute.OrganizationAccessPolicy{httproute.OrganizationAccessPolicyCachedRead, httproute.OrganizationAccessPolicyCachedRead, httproute.OrganizationAccessPolicyCachedRead}, r.policies)
	_, err = a.Authorize(ctx, authz.PermissionWorkbenchToolsManage, false)
	require.ErrorIs(t, err, tm.ErrForbidden)
	require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, r.policies[len(r.policies)-1])
}

func TestCachedReadKeepsOriginalActorTokenAndSelectedOrganization(t *testing.T) {
	for name, mutate := range map[string]func(*authidentity.AuthenticatedIdentity){
		"actor":        func(id *authidentity.AuthenticatedIdentity) { id.UserID = "other" },
		"expiry":       func(id *authidentity.AuthenticatedIdentity) { id.TokenExpiresAt = id.TokenExpiresAt.Add(time.Hour) },
		"tenant":       func(id *authidentity.AuthenticatedIdentity) { id.TenantID = "other" },
		"organization": func(id *authidentity.AuthenticatedIdentity) { id.EffectiveOrganizationID = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			a, err := New(&policyResolver{mutate: mutate}, policies{})
			require.NoError(t, err)
			id := authidentity.AuthenticatedIdentity{UserID: "actor", EffectiveOrganizationID: "org", TokenExpiresAt: time.Now().Add(time.Hour)}
			ctx, err := a.Bind(authidentity.WithAuthenticatedIdentity(context.Background(), id), "Bearer verified-token")
			require.NoError(t, err)
			_, err = a.AuthorizeRead(ctx, authz.PermissionWorkbenchToolsRead, false)
			require.ErrorIs(t, err, tm.ErrForbidden)
		})
	}
}

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
	scope, e = a.AuthorizeRead(ctx, authz.PermissionListingKitPlatformAdm, true)
	require.NoError(t, e)
	require.Equal(t, tm.Scope{ActorID: "specialist"}, scope)
	_, e = a.Authorize(ctx, authz.PermissionWorkbenchToolsManage, true)
	require.ErrorIs(t, e, tm.ErrForbidden)
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{UserID: "operator", Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Hour)})
	_, e = a.Authorize(ctx, authz.PermissionListingKitPlatformAdm, true)
	require.ErrorIs(t, e, tm.ErrForbidden)
}
