package reportcenterapp

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	rc "task-processor/internal/reportcenter"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

type resolveFunc func(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error)

func (f resolveFunc) Resolve(c context.Context, p httproute.OrganizationAccessPolicy, i workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	return f(c, p, i)
}
func TestReportAuthorizationPreservesConfiguredUserAuthority(t *testing.T) {
	policy, err := authz.NewListingKitAuthorizer([]string{"configured-user"}, nil)
	require.NoError(t, err)
	for _, user := range []string{"configured-user", "unconfigured-user"} {
		id := authidentity.AuthenticatedIdentity{UserID: user, TenantID: "org-a", EffectiveOrganizationID: "org-a", TokenExpiresAt: time.Now().Add(time.Hour)}
		a := &authorization{resolver: resolveFunc(func(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
			return id, nil
		}), policy: policy}
		ctx, err := a.Bind(authidentity.WithAuthenticatedIdentity(context.Background(), id), "Bearer fixture-token")
		require.NoError(t, err)
		for _, manage := range []bool{false, true} {
			_, err := a.Authorize(ctx, rc.Scope{OrganizationID: "org-a", ActorID: user}, manage)
			if user == "configured-user" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, rc.ErrForbidden)
			}
		}
	}
}
func TestAuthorizationUsesOriginalProofAndResolvedScopedRoles(t *testing.T) {
	identity := authidentity.AuthenticatedIdentity{UserID: "actor-a", TenantID: "org-a", EffectiveOrganizationID: "org-a", HomeOrganizationID: "home-a", TokenExpiresAt: time.Now().Add(time.Hour)}
	scope := rc.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	calls := []httproute.OrganizationAccessPolicy{}
	a := &authorization{resolver: resolveFunc(func(_ context.Context, p httproute.OrganizationAccessPolicy, in workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
		require.Equal(t, "private-test-bearer", in.BearerToken)
		require.Equal(t, identity.UserID, in.Identity.UserID)
		require.Empty(t, in.Identity.Roles)
		calls = append(calls, p)
		id := identity
		id.Roles = []string{"listingkit_operator"}
		return id, nil
	}), policy: allowPolicy(true)}
	bound, e := a.Bind(authidentity.WithAuthenticatedIdentity(context.Background(), identity), "Bearer private-test-bearer")
	require.NoError(t, e)
	fresh, e := a.Authorize(bound, scope, true)
	require.NoError(t, e)
	got, ok := authidentity.AuthenticatedIdentityFromContext(fresh)
	require.True(t, ok)
	require.Equal(t, []string{"listingkit_operator"}, got.Roles)
	_, e = a.Authorize(bound, scope, false)
	require.NoError(t, e)
	require.Equal(t, []httproute.OrganizationAccessPolicy{httproute.OrganizationAccessPolicyLiveWrite, httproute.OrganizationAccessPolicyCachedRead}, calls)
	for _, drift := range []string{"actor", "org", "expiry"} {
		a.resolver = resolveFunc(func(context.Context, httproute.OrganizationAccessPolicy, workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
			id := identity
			switch drift {
			case "actor":
				id.UserID = "other"
			case "org":
				id.TenantID = "other"
			case "expiry":
				id.TokenExpiresAt = id.TokenExpiresAt.Add(time.Hour)
			}
			return id, nil
		})
		_, e = a.Authorize(bound, scope, true)
		require.ErrorIs(t, e, rc.ErrForbidden)
	}
	_, e = a.Bind(context.Background(), "Bearer private-test-bearer")
	require.ErrorIs(t, e, rc.ErrForbidden)
}
