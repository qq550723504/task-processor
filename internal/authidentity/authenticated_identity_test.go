package authidentity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/shared/aiidentity"
)

func TestAuthenticatedIdentityRoundTripsThroughContext(t *testing.T) {
	roles := []string{"listingkit_operator"}
	grantRoles := []string{"listingkit_admin"}
	grants := []OrganizationGrant{{
		OrganizationID:   " org-a ",
		OrganizationName: " Organization A ",
		ProjectID:        " project-1 ",
		Roles:            grantRoles,
	}}
	expiresAt := time.Date(2026, time.August, 30, 12, 30, 0, 0, time.FixedZone("SGT", 8*60*60))
	ctx := WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{
		TenantID:                " tenant-a ",
		UserID:                  " user-a ",
		Roles:                   roles,
		HomeOrganizationID:      " org-home ",
		EffectiveOrganizationID: " org-a ",
		OrganizationGrants:      grants,
		TokenExpiresAt:          expiresAt,
	})
	roles[0] = "mutated-after-storage"
	grantRoles[0] = "mutated-after-storage"
	grants[0].OrganizationID = "mutated-after-storage"

	want := AuthenticatedIdentity{
		TenantID:                "tenant-a",
		UserID:                  "user-a",
		Roles:                   []string{"listingkit_operator"},
		HomeOrganizationID:      "org-home",
		EffectiveOrganizationID: "org-a",
		OrganizationGrants: []OrganizationGrant{{
			OrganizationID:   "org-a",
			OrganizationName: "Organization A",
			ProjectID:        "project-1",
			Roles:            []string{"listingkit_admin"},
		}},
		TokenExpiresAt: expiresAt.UTC(),
	}

	got, ok := AuthenticatedIdentityFromContext(ctx)

	require.True(t, ok)
	require.Equal(t, want, got)
	require.Equal(t, aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"}, aiidentity.FromContext(ctx))
	got.Roles[0] = "mutated-after-read"
	got.OrganizationGrants[0].OrganizationID = "mutated-after-read"
	got.OrganizationGrants[0].Roles[0] = "mutated-after-read"
	got, ok = AuthenticatedIdentityFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, want, got)
}

func TestAuthenticatedIdentityKeepsEffectiveRolesScopedToSelectedOrganization(t *testing.T) {
	ctx := WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{
		TenantID:                "org-b",
		UserID:                  "user-a",
		Roles:                   []string{"listingkit_viewer"},
		HomeOrganizationID:      "org-a",
		EffectiveOrganizationID: "org-b",
		OrganizationGrants: []OrganizationGrant{
			{OrganizationID: "org-a", ProjectID: "project-1", Roles: []string{"listingkit_admin"}},
			{OrganizationID: "org-b", ProjectID: "project-1", Roles: []string{"listingkit_viewer"}},
		},
	})

	got, ok := AuthenticatedIdentityFromContext(ctx)

	require.True(t, ok)
	require.Equal(t, "org-b", got.EffectiveOrganizationID)
	require.Equal(t, []string{"listingkit_viewer"}, got.Roles)
	require.Equal(t, []string{"listingkit_admin"}, got.OrganizationGrants[0].Roles)
	require.Equal(t, []string{"listingkit_viewer"}, got.OrganizationGrants[1].Roles)
}

func TestAuthenticatedIdentityFromContextRejectsIncompleteIdentity(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing identity", ctx: context.Background()},
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

func TestAuthenticatedIdentityFromContextAcceptsVerifiedUserWithoutEffectiveOrganization(t *testing.T) {
	ctx := WithAuthenticatedIdentity(context.Background(), AuthenticatedIdentity{
		UserID:             " user-a ",
		HomeOrganizationID: " org-home ",
		Roles:              []string{},
	})

	got, ok := AuthenticatedIdentityFromContext(ctx)

	require.True(t, ok)
	require.Equal(t, "user-a", got.UserID)
	require.Empty(t, got.TenantID)
	require.Empty(t, got.EffectiveOrganizationID)
	require.Empty(t, got.Roles)
}
