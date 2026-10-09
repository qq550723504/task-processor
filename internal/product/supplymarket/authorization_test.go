package supplymarket

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/product/collection"
	"testing"
	"time"
)

type liveRoles struct{}

func (liveRoles) ResolveLiveRoles(context.Context, string, string) ([]string, error) {
	return []string{"owner"}, nil
}
func TestPlatformAuthorityRequiresVerifiedGlobalContextAndUnexpiredIdentity(t *testing.T) {
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	a, err := NewContextAuthorizer(liveRoles{}, permissions)
	require.NoError(t, err)
	identity := authidentity.AuthenticatedIdentity{UserID: "platform-a", Roles: []string{"platform_admin"}, TokenExpiresAt: time.Now().Add(time.Minute)}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
	actor, err := a.AuthorizePlatform(ctx)
	require.NoError(t, err)
	require.Equal(t, identity.UserID, actor)
	identity.TenantID = "org-a"
	identity.EffectiveOrganizationID = "org-a"
	identity.EffectiveMemberID = "member-a"
	_, err = a.AuthorizePlatform(authidentity.WithAuthenticatedIdentity(ctx, identity))
	require.ErrorIs(t, err, ErrForbidden)
	identity = authidentity.AuthenticatedIdentity{UserID: "member-a", Roles: []string{"admin", "owner"}, TokenExpiresAt: time.Now().Add(time.Minute)}
	_, err = a.AuthorizePlatform(authidentity.WithAuthenticatedIdentity(ctx, identity))
	require.ErrorIs(t, err, ErrForbidden)
	identity.Roles = []string{"platform_admin"}
	identity.TokenExpiresAt = time.Now().Add(-time.Second)
	_, err = a.AuthorizePlatform(authidentity.WithAuthenticatedIdentity(ctx, identity))
	require.ErrorIs(t, err, ErrForbidden)
	_, err = a.Authorize(nil, collection.PermissionManage)
	require.ErrorIs(t, err, ErrForbidden)
}
