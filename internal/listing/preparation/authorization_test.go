package preparation

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"testing"
	"time"
)

type liveAccess func(context.Context, string, string) ([]string, error)

func (f liveAccess) ResolveLiveRoles(ctx context.Context, org, user string) ([]string, error) {
	return f(ctx, org, user)
}

func TestPreparationAuthorizationUsesCurrentRoleAndRechecksExpiry(t *testing.T) {
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	now := time.Now()
	roles := []string{"listingkit_viewer"}
	advance := false
	auth, err := NewContextAuthorizer(liveAccess(func(context.Context, string, string) ([]string, error) {
		if advance {
			now = now.Add(time.Minute)
		}
		return roles, nil
	}), permissions)
	require.NoError(t, err)
	auth.now = func() time.Time { return now }
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member", TokenExpiresAt: now.Add(10 * time.Second)})
	_, err = auth.Authorize(ctx, PermissionRead)
	require.ErrorIs(t, err, ErrForbidden, "retired static viewer roles do not replace current enterprise module grants")
	_, err = auth.Authorize(ctx, PermissionManage)
	require.ErrorIs(t, err, ErrForbidden)
	_, err = auth.Authorize(ctx, PermissionSubmit)
	require.ErrorIs(t, err, ErrForbidden)
	roles = []string{"listingkit_admin"}
	_, err = auth.Authorize(ctx, PermissionRead)
	require.NoError(t, err)
	_, err = auth.Authorize(ctx, PermissionSubmit)
	require.NoError(t, err)
	roles = nil
	_, err = auth.Authorize(ctx, PermissionRead)
	require.ErrorIs(t, err, ErrForbidden)
	roles = []string{"listingkit_admin"}
	advance = true
	_, err = auth.Authorize(ctx, PermissionManage)
	require.ErrorIs(t, err, ErrForbidden)
}
