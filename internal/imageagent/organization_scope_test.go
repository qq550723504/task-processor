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

func TestOrganizationRunIdentityRejectsReplacementMemberGrant(t *testing.T) {
	service := &Service{organizationScope: true}
	run := Run{ScopeProtocol: OrganizationScopeProtocol, ID: "run-1", TenantID: "org-1", UserID: "actor-1", MemberID: "old-member", BusinessTaskID: "operation-1"}
	identity := ExecutionIdentity{ScopeProtocol: OrganizationScopeProtocol, TenantID: "org-1", UserID: "actor-1", MemberID: "new-member"}
	_, err := service.identityForRun(identity, run)
	require.ErrorIs(t, err, ErrIdentityRequired)
	identity.MemberID = run.MemberID
	_, err = service.identityForRun(identity, run)
	require.NoError(t, err)
}
