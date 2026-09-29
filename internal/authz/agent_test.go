package authz

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAgentPermissionsRemainIndependent(t *testing.T) {
	a := DefaultListingKitAuthorizer()
	for _, row := range []struct {
		role                 string
		read, use, configure bool
	}{{"listingkit_viewer", true, false, false}, {"listingkit_operator", true, true, false}, {"listingkit_admin", true, true, true}, {"platform_admin", true, true, true}, {"viewer", false, false, false}} {
		require.Equal(t, row.read, a.Authorize("", []string{row.role}, PermissionWorkbenchAgentRead), row.role)
		require.Equal(t, row.use, a.Authorize("", []string{row.role}, PermissionWorkbenchAgentUse), row.role)
		require.Equal(t, row.configure, a.Authorize("", []string{row.role}, PermissionWorkbenchAgentConfigure), row.role)
	}
}
