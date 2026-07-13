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
