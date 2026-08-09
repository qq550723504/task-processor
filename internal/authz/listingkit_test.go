package authz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListingKitAuthorizerAllowsOperationalRolesToWriteProductSourcing(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)

	require.True(t, authorizer.Authorize("", []string{"listingkit_operator"}, PermissionProductSourcingWrite))
	require.True(t, authorizer.Authorize("", []string{"listingkit_admin"}, PermissionProductSourcingWrite))
	require.True(t, authorizer.Authorize("", []string{"platform_admin"}, PermissionProductSourcingWrite))
	require.False(t, authorizer.Authorize("", []string{"viewer"}, PermissionProductSourcingWrite))
}

func TestListingKitAuthorizerDoesNotTreatListingKitAdminAsPlatformAdmin(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)

	require.False(t, authorizer.Authorize("", []string{"listingkit_admin"}, PermissionListingKitPlatformAdm))
	require.True(t, authorizer.Authorize("", []string{"platform_admin"}, PermissionListingKitPlatformAdm))
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
