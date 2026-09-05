package authz

import (
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
)

func TestListingKitAuthorizerAllowsOperationalRolesToWriteProductSourcing(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)

	require.True(t, authorizer.Authorize("", []string{"listingkit_operator"}, PermissionProductSourcingWrite))
	require.True(t, authorizer.Authorize("", []string{"listingkit_operator"}, PermissionLocalAgentWrite))
	require.True(t, authorizer.Authorize("", []string{"listingkit_admin"}, PermissionProductSourcingWrite))
	require.True(t, authorizer.Authorize("", []string{"listingkit_admin"}, PermissionLocalAgentWrite))
	require.True(t, authorizer.Authorize("", []string{"platform_admin"}, PermissionProductSourcingWrite))
	require.False(t, authorizer.Authorize("", []string{"viewer"}, PermissionProductSourcingWrite))
}

func TestListingKitAuthorizerEnforcesImageAgentReadAndWritePermissions(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, err)
	for _, role := range []string{"listingkit_operator", "listingkit_admin", "platform_admin", "admin", "configured-role"} {
		require.True(t, authorizer.Authorize("", []string{role}, PermissionImageAgentRead), role)
		require.True(t, authorizer.Authorize("", []string{role}, PermissionImageAgentWrite), role)
	}
	require.True(t, authorizer.Authorize("configured-user", nil, PermissionImageAgentRead))
	require.True(t, authorizer.Authorize("configured-user", nil, PermissionImageAgentWrite))
	require.False(t, authorizer.Authorize("", []string{"viewer"}, PermissionImageAgentRead))
	require.False(t, authorizer.Authorize("", []string{"viewer"}, PermissionImageAgentWrite))
}

func TestListingKitAuthorizerGrantsLocalAgentToConfiguredAdmins(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer([]string{"user-1"}, []string{"custom_platform_admin"})
	require.NoError(t, err)
	require.True(t, authorizer.Authorize("user-1", nil, PermissionLocalAgentWrite))
	require.True(t, authorizer.Authorize("", []string{"custom_platform_admin"}, PermissionLocalAgentWrite))
	require.True(t, authorizer.Authorize("", []string{"admin"}, PermissionLocalAgentWrite))
}

func TestListingKitAuthorizerDoesNotTreatListingKitAdminAsPlatformAdmin(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)

	require.False(t, authorizer.Authorize("", []string{"listingkit_admin"}, PermissionListingKitPlatformAdm))
	require.True(t, authorizer.Authorize("", []string{"platform_admin"}, PermissionListingKitPlatformAdm))
}

func TestListingKitAuthorizerUsesConfiguredPlatformAdminSemanticsConsistently(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, err)

	for _, identity := range []struct {
		userID string
		roles  []string
	}{
		{userID: "configured-user"},
		{roles: []string{"configured-role"}},
	} {
		require.True(t, authorizer.Authorize(identity.userID, identity.roles, PermissionListingKitAdminRead))
		require.True(t, authorizer.IsTenantAdmin(identity.userID, identity.roles))
	}
	require.True(t, authorizer.Authorize("", []string{"admin"}, PermissionListingKitAdminRead))
	require.True(t, authorizer.IsTenantAdmin("", []string{"admin"}))
}

func TestListingKitAuthorizerRestrictsPromptWritesToTenantAndPlatformAdmins(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)

	require.False(t, authorizer.Authorize("", []string{"listingkit_viewer"}, "listingkit.prompt.write"))
	require.False(t, authorizer.Authorize("", []string{"listingkit_operator"}, "listingkit.prompt.write"))
	require.True(t, authorizer.Authorize("", []string{"listingkit_admin"}, "listingkit.prompt.write"))
	require.True(t, authorizer.Authorize("", []string{"platform_admin"}, "listingkit.prompt.write"))
}

func TestIsListingKitTenantAdminIncludesTenantAndPlatformAdmins(t *testing.T) {
	require.True(t, IsListingKitTenantAdmin("", []string{"listingkit_admin"}))
	require.True(t, IsListingKitTenantAdmin("", []string{"platform_admin"}))
	require.True(t, IsListingKitTenantAdmin("", []string{"admin"}))
	require.False(t, IsListingKitTenantAdmin("", []string{"listingkit_operator"}))
}

func TestListingKitAuthorizerEnforcesScopedStorePermissionMatrix(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, err)
	permissions := []string{
		PermissionWorkbenchStoreRead,
		PermissionWorkbenchStoreCreate,
		PermissionWorkbenchStoreUpdate,
		PermissionWorkbenchStoreLifecycle,
		PermissionWorkbenchStoreDelete,
	}
	tests := []struct {
		role string
		want []bool
	}{
		{role: "listingkit_viewer", want: []bool{true, false, false, false, false}},
		{role: "listingkit_operator", want: []bool{true, true, true, true, false}},
		{role: "listingkit_admin", want: []bool{true, true, true, true, true}},
		{role: "platform_admin", want: []bool{true, true, true, true, true}},
		{role: "admin", want: []bool{false, false, false, false, false}},
		{role: "configured-role", want: []bool{true, true, true, true, true}},
	}
	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			for index, permission := range permissions {
				require.Equal(t, tt.want[index], authorizer.Authorize("", []string{tt.role}, permission), permission)
			}
		})
	}
	for _, permission := range permissions {
		require.True(t, authorizer.Authorize("configured-user", nil, permission), permission)
	}
}

func TestListingKitAuthorizerUsesOnlyEffectiveOrganizationRolesForStorePermissions(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)

	// The authorization call deliberately consumes only the effective
	// Organization B roles, never the Organization A grants.
	identity := authidentity.AuthenticatedIdentity{
		UserID: "user-1", EffectiveOrganizationID: "org-b", Roles: []string{"listingkit_viewer"},
		OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", Roles: []string{"listingkit_admin"}}},
	}
	require.True(t, authorizer.Authorize(identity.UserID, identity.Roles, PermissionWorkbenchStoreRead))
	require.False(t, authorizer.Authorize(identity.UserID, identity.Roles, PermissionWorkbenchStoreCreate))
	require.False(t, authorizer.Authorize(identity.UserID, identity.Roles, PermissionWorkbenchStoreDelete))
}
