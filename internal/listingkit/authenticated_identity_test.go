package listingkit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthenticatedIdentityRoundTripsThroughContext(t *testing.T) {
	want := AuthenticatedIdentity{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Roles:    []string{"listingkit_operator"},
	}

	got, ok := AuthenticatedIdentityFromContext(WithAuthenticatedIdentity(context.Background(), want))

	require.True(t, ok)
	require.Equal(t, want, got)
}

func TestAuthenticatedIdentityFromContextRejectsMissingOrBlankTenant(t *testing.T) {
	_, ok := AuthenticatedIdentityFromContext(context.Background())
	require.False(t, ok)

	_, ok = AuthenticatedIdentityFromContext(WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{UserID: "user-a"}))
	require.False(t, ok)
}
