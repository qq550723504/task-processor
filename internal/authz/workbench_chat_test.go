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

func TestConfiguredPlatformAdminChatUseHasTitleExecutionPrerequisites(t *testing.T) {
	a, err := NewListingKitAuthorizer(nil, []string{"custom_platform_admin"})
	require.NoError(t, err)
	roles := []string{"custom_platform_admin"}
	require.True(t, a.Authorize("", roles, PermissionWorkbenchChatUse))
	require.True(t, a.Authorize("", roles, PermissionWorkbenchAgentUse))
	require.True(t, a.Authorize("", roles, PermissionListingKitAdminWrite))
	require.False(t, a.Authorize("", []string{"listingkit_viewer"}, PermissionWorkbenchChatUse))
}

func TestConfiguredPlatformAdminUserChatUseHasTitleExecutionPrerequisites(t *testing.T) {
	a, err := NewListingKitAuthorizer([]string{"custom-platform-user"}, nil)
	require.NoError(t, err)
	for _, permission := range []string{
		PermissionWorkbenchChatRead, PermissionWorkbenchChatUse, PermissionWorkbenchTaskRead,
		PermissionWorkbenchAgentUse, PermissionListingKitAdminWrite,
	} {
		require.True(t, a.Authorize("custom-platform-user", nil, permission), permission)
		require.False(t, a.Authorize("other-user", nil, permission), permission)
	}
}
