package authz

import (
	"context"
	"github.com/stretchr/testify/require"
	pc "task-processor/internal/aiworkbench/projectcenter"
	"testing"
)

func TestProjectCenterRolesReuseConfiguredPlatformAuthority(t *testing.T) {
	a, e := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, e)
	for _, subject := range []struct {
		user   string
		roles  []string
		manage bool
	}{{"actor", []string{"listingkit_viewer"}, false}, {"actor", []string{"listingkit_operator"}, true}, {"actor", []string{"configured-role"}, true}, {"configured-user", nil, true}} {
		require.True(t, AllowedOrganization(context.Background(), a, subject.user, "org-a", subject.roles, pc.PermissionRead))
		require.Equal(t, subject.manage, AllowedOrganization(context.Background(), a, subject.user, "org-a", subject.roles, pc.PermissionManage))
	}
	require.False(t, AllowedOrganization(context.Background(), a, "other", "org-a", nil, pc.PermissionManage))
	require.Contains(t, WorkbenchPermissions(), pc.PermissionRead)
	require.Contains(t, ModulePermissions([]string{"projects"}), pc.PermissionManage)
}
