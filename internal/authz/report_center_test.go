package authz

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReportCenterPoliciesAndModuleCannotGrantOriginalSources(t *testing.T) {
	a, err := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, err)
	for _, role := range []string{"listingkit_admin", "platform_admin", "configured-role"} {
		require.True(t, AllowedOrganization(context.Background(), a, "actor", "org-a", []string{role}, "workbench.report.read"))
		require.True(t, AllowedOrganization(context.Background(), a, "actor", "org-a", []string{role}, "workbench.report.manage"))
	}
	require.True(t, a.Authorize("configured-user", nil, "workbench.report.manage"))
	require.True(t, a.Authorize("actor", []string{"listingkit_viewer"}, "workbench.report.read"))
	require.False(t, a.Authorize("actor", []string{"listingkit_viewer"}, "workbench.report.manage"))
	key := EnterpriseRoleKey("org-a", 1)
	policy := rolePolicyFixture{"org-a": {key: {"reports"}}}
	a.SetRolePolicyReader(policy)
	for _, permission := range []string{"workbench.report.read", "workbench.report.manage"} {
		require.True(t, AllowedOrganization(context.Background(), a, "actor", "org-a", []string{key}, permission))
		require.False(t, AllowedOrganization(context.Background(), a, "actor", "org-b", []string{key}, permission))
	}
	for _, permission := range []string{PermissionLocalAgentWrite, PermissionListingKitAdminRead, PermissionListingKitPlatformAdm, PermissionWorkbenchAgentUse} {
		require.False(t, AllowedOrganization(context.Background(), a, "actor", "org-a", []string{key}, permission))
	}
	permissions, err := a.ScopedPermissions(context.Background(), "actor", "org-a", []string{key})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"workbench.report.read", "workbench.report.manage"}, permissions)
	policy["org-a"][key] = nil
	require.False(t, AllowedOrganization(context.Background(), a, "actor", "org-a", []string{key}, "workbench.report.manage"))
}
