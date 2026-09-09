package sourcing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type liveRolesFunc func(context.Context, string, string) ([]string, error)

func (f liveRolesFunc) ResolveLiveRoles(ctx context.Context, organizationID, actorID string) ([]string, error) {
	return f(ctx, organizationID, actorID)
}

func publicationIdentityContext(identity authidentity.AuthenticatedIdentity) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), identity)
}

func TestContextAuthorizerUsesTrustedEffectiveOrganizationAndFreshRoles(t *testing.T) {
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	calls := 0
	authorizer, err := NewContextAuthorizer(liveRolesFunc(func(_ context.Context, org, actor string) ([]string, error) {
		calls++
		require.Equal(t, "org-a", org)
		require.Equal(t, "actor-a", actor)
		return []string{"listingkit_operator"}, nil
	}), permissions)
	require.NoError(t, err)

	scope, err := authorizer.Authorize(publicationIdentityContext(authidentity.AuthenticatedIdentity{
		TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor-a",
		Roles: []string{"viewer"}, TokenExpiresAt: time.Now().Add(time.Hour),
	}))
	require.NoError(t, err)
	require.Equal(t, PublicationScope{OrganizationID: "org-a", ActorID: "actor-a"}, scope)
	require.Equal(t, 1, calls)
}

func TestContextAuthorizerHonorsConfiguredPlatformAdministrators(t *testing.T) {
	permissions, err := authz.NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, err)
	for _, test := range []struct {
		name   string
		userID string
		roles  []string
	}{
		{name: "configured user", userID: "configured-user"},
		{name: "configured role", userID: "actor-a", roles: []string{"configured-role"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			authorizer, newErr := NewContextAuthorizer(liveRolesFunc(func(_ context.Context, org, actor string) ([]string, error) {
				require.Equal(t, "org-a", org)
				require.Equal(t, test.userID, actor)
				return test.roles, nil
			}), permissions)
			require.NoError(t, newErr)
			scope, authorizeErr := authorizer.Authorize(publicationIdentityContext(authidentity.AuthenticatedIdentity{
				TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: test.userID, TokenExpiresAt: time.Now().Add(time.Hour),
			}))
			require.NoError(t, authorizeErr)
			require.Equal(t, PublicationScope{OrganizationID: "org-a", ActorID: test.userID}, scope)
		})
	}
}

func TestContextAuthorizerRejectsInvalidContextRevocationAndDependencyFailure(t *testing.T) {
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	dependencyErr := errors.New("grant dependency unavailable")

	tests := []struct {
		name     string
		identity authidentity.AuthenticatedIdentity
		roles    []string
		liveErr  error
		wantErr  error
		wantLive bool
	}{
		{name: "missing", wantErr: ErrPublicationForbidden},
		{name: "cross organization", identity: authidentity.AuthenticatedIdentity{TenantID: "org-b", EffectiveOrganizationID: "org-a", UserID: "actor", TokenExpiresAt: time.Now().Add(time.Hour)}, wantErr: ErrPublicationForbidden},
		{name: "expired", identity: authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor", TokenExpiresAt: time.Now().Add(-time.Second)}, wantErr: ErrPublicationForbidden},
		{name: "revoked permission", identity: authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor", TokenExpiresAt: time.Now().Add(time.Hour)}, roles: []string{"viewer"}, wantErr: ErrPublicationForbidden, wantLive: true},
		{name: "dependency failure", identity: authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: "actor", TokenExpiresAt: time.Now().Add(time.Hour)}, liveErr: dependencyErr, wantErr: dependencyErr, wantLive: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			liveCalls := 0
			authorizer, newErr := NewContextAuthorizer(liveRolesFunc(func(context.Context, string, string) ([]string, error) {
				liveCalls++
				return test.roles, test.liveErr
			}), permissions)
			require.NoError(t, newErr)
			_, gotErr := authorizer.Authorize(publicationIdentityContext(test.identity))
			require.ErrorIs(t, gotErr, test.wantErr)
			if test.wantLive {
				require.Equal(t, 1, liveCalls)
			} else {
				require.Zero(t, liveCalls)
			}
		})
	}
}
