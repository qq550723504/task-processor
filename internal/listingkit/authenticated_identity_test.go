package listingkit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/shared/aiidentity"
)

func TestAuthenticatedIdentityRoundTripsThroughContext(t *testing.T) {
	roles := []string{"listingkit_operator"}
	ctx := WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{
		TenantID: " tenant-a ",
		UserID:   " user-a ",
		Roles:    roles,
	})
	roles[0] = "mutated-after-storage"

	want := AuthenticatedIdentity{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Roles:    []string{"listingkit_operator"},
	}

	got, ok := AuthenticatedIdentityFromContext(ctx)

	require.True(t, ok)
	require.Equal(t, want, got)
	require.Equal(t, aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"}, aiidentity.FromContext(ctx))
	got.Roles[0] = "mutated-after-read"
	got, ok = AuthenticatedIdentityFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, want, got)
}

func TestAuthenticatedIdentityFromContextRejectsIncompleteIdentity(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing identity", ctx: context.Background()},
		{name: "missing tenant", ctx: WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{UserID: "user-a"})},
		{name: "blank tenant", ctx: WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{TenantID: " \t ", UserID: "user-a"})},
		{name: "missing user", ctx: WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{TenantID: "tenant-a"})},
		{name: "blank user", ctx: WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{TenantID: "tenant-a", UserID: " \t "})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := AuthenticatedIdentityFromContext(tt.ctx)
			require.False(t, ok)
		})
	}
}
