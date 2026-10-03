package authz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkbenchChatAndTaskPermissionsAreExplicit(t *testing.T) {
	a, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	for _, tc := range []struct {
		role            string
		read, use, task bool
	}{
		{"listingkit_viewer", true, false, true},
		{"listingkit_operator", true, true, true},
		{"listingkit_admin", true, true, true},
		{"platform_admin", true, true, true},
		{"unknown", false, false, false},
	} {
		require.Equal(t, tc.read, a.Authorize("", []string{tc.role}, PermissionWorkbenchChatRead), tc.role)
		require.Equal(t, tc.use, a.Authorize("", []string{tc.role}, PermissionWorkbenchChatUse), tc.role)
		require.Equal(t, tc.task, a.Authorize("", []string{tc.role}, PermissionWorkbenchTaskRead), tc.role)
	}
}
