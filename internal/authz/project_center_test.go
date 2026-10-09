package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	pc "task-processor/internal/aiworkbench/projectcenter"
)

func TestProjectCenterRolesReuseConfiguredPlatformAuthority(t *testing.T) {
	a, e := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, e)
	for _, subject := range []struct {
		user   string
		roles  []string
		manage bool
	}{{"actor", []string{"listingkit_admin"}, true}, {"actor", []string{"platform_admin"}, true}, {"actor", []string{"configured-role"}, true}, {"configured-user", nil, true}} {
		require.True(t, AllowedOrganization(context.Background(), a, subject.user, "org-a", subject.roles, pc.PermissionRead))
		require.Equal(t, subject.manage, AllowedOrganization(context.Background(), a, subject.user, "org-a", subject.roles, pc.PermissionManage))
	}
	// Native viewer/operator policies remain available to static consumers;
	// organization policy deliberately requires their current enterprise roles.
	require.True(t, a.Authorize("actor", []string{"listingkit_viewer"}, pc.PermissionRead))
	require.False(t, a.Authorize("actor", []string{"listingkit_viewer"}, pc.PermissionManage))
	require.True(t, a.Authorize("actor", []string{"listingkit_operator"}, pc.PermissionManage))
	require.False(t, AllowedOrganization(context.Background(), a, "actor", "org-a", []string{"listingkit_operator"}, pc.PermissionManage))
	require.False(t, AllowedOrganization(context.Background(), a, "other", "org-a", nil, pc.PermissionManage))
	require.Contains(t, WorkbenchPermissions(), pc.PermissionRead)
	require.Contains(t, ModulePermissions([]string{"projects"}), pc.PermissionManage)
}
