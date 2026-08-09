package listingkit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/shared/aiidentity"
)

func TestAuthenticatedIdentityRoundTripsThroughContext(t *testing.T) {
	want := AuthenticatedIdentity{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Roles:    []string{"listingkit_operator"},
	}

	ctx := WithAuthenticatedIdentity(context.Background(), want)
	got, ok := AuthenticatedIdentityFromContext(ctx)

	require.True(t, ok)
	require.Equal(t, want, got)
	require.Equal(t, aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"}, aiidentity.FromContext(ctx))
}

func TestAuthenticatedIdentityFromContextRejectsMissingOrBlankTenant(t *testing.T) {
	_, ok := AuthenticatedIdentityFromContext(context.Background())
	require.False(t, ok)

	_, ok = AuthenticatedIdentityFromContext(WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{UserID: "user-a"}))
	require.False(t, ok)
}
