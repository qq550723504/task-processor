package authz

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSupplyPermissionsAreExplicitAndDoNotGrantPublishingToViewers(t *testing.T) {
	authorizer, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	for _, permission := range []string{PermissionWorkbenchCollectionRead, PermissionWorkbenchCollectionManage, PermissionWorkbenchSupplyRead, PermissionWorkbenchSupplyManage, PermissionWorkbenchListingSubmit} {
		require.True(t, authorizer.Authorize("actor", []string{"listingkit_operator"}, permission))
		require.True(t, authorizer.Authorize("actor", []string{"listingkit_admin"}, permission))
	}
	require.True(t, authorizer.Authorize("actor", []string{"listingkit_viewer"}, PermissionWorkbenchCollectionRead))
	require.True(t, authorizer.Authorize("actor", []string{"listingkit_viewer"}, PermissionWorkbenchSupplyRead))
	require.False(t, authorizer.Authorize("actor", []string{"listingkit_viewer"}, PermissionWorkbenchCollectionManage))
	require.False(t, authorizer.Authorize("actor", []string{"listingkit_viewer"}, PermissionWorkbenchListingSubmit))
	require.False(t, authorizer.Authorize("actor", []string{"unassigned"}, PermissionWorkbenchListingSubmit))
}
