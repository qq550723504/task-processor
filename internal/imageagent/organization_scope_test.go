package imageagent

import (
	"context"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"testing"
)

func TestOrganizationIdentityRequiresResolvedScope(t *testing.T) {
	service := &Service{organizationScope: true}
	for _, identity := range []authidentity.AuthenticatedIdentity{
		{TenantID: "home-a", UserID: "actor", HomeOrganizationID: "home-a"},
		{TenantID: "home-a", UserID: "actor", EffectiveOrganizationID: "org-b"},
	} {
		_, err := service.executionIdentity(authidentity.WithAuthenticatedIdentity(context.Background(), identity))
		require.Error(t, err)
	}
	identity, err := service.executionIdentity(authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org-b", EffectiveOrganizationID: "org-b", HomeOrganizationID: "home-a", UserID: "actor"}))
	require.NoError(t, err)
	require.Equal(t, "org-b", identity.TenantID)
	require.Equal(t, OrganizationScopeProtocol, identity.ScopeProtocol)
}
