package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolMarketStaticPoliciesDoNotReviveRetiredMemberRoles(t *testing.T) {
	a, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	for _, policy := range ToolMarketPolicies() {
		_, err = a.enforcer.AddPolicy(policy)
		require.NoError(t, err)
	}
	for _, role := range []string{"listingkit_viewer", "listingkit_operator"} {
		for _, permission := range []string{PermissionWorkbenchToolsRead, PermissionWorkbenchToolsManage, PermissionWorkbenchToolsCustomize} {
			require.False(t, a.Authorize("member", []string{role}, permission), "retired role %s must not gain %s", role, permission)
		}
	}
	require.True(t, a.Authorize("admin", []string{"listingkit_admin"}, PermissionWorkbenchToolsManage))
}

func TestToolMarketNativeModuleGrantsUseRealScopedCasbinWithoutEscalation(t *testing.T) {
	// This fixture represents the shared Writer's planned catalog wiring. The
	// current installation is not changed or declared ready by this test.
	original := enterpriseModules
	enterpriseModules = MenuModules()
	t.Cleanup(func() { enterpriseModules = original })
	for i := range enterpriseModules {
		if permissions := ToolMarketModulePermissions(enterpriseModules[i].ID); len(permissions) > 0 {
			enterpriseModules[i].Available = true
			enterpriseModules[i].Permissions = permissions
		}
	}
	ctx := context.Background()
	readKey, customizeKey := EnterpriseRoleKey("org-a", 1), EnterpriseRoleKey("org-a", 2)
	policy := rolePolicyFixture{"org-a": {readKey: {"tools"}, customizeKey: {"tools-custom"}}}
	a, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	for _, p := range ToolMarketPolicies() {
		_, err = a.enforcer.AddPolicy(p)
		require.NoError(t, err)
	}
	a.SetRolePolicyReader(policy)
	for _, key := range []string{readKey, customizeKey} {
		require.True(t, AllowedOrganization(ctx, a, "member", "org-a", []string{key}, PermissionWorkbenchToolsRead))
		for _, permission := range []string{PermissionWorkbenchToolsManage, PermissionListingKitPlatformAdm} {
			require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{key}, permission))
		}
		require.False(t, AllowedOrganization(ctx, a, "member", "org-b", []string{key}, PermissionWorkbenchToolsRead))
	}
	require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{readKey}, PermissionWorkbenchToolsCustomize))
	require.True(t, AllowedOrganization(ctx, a, "member", "org-a", []string{customizeKey}, PermissionWorkbenchToolsCustomize))
	for _, role := range []string{"listingkit_viewer", "listingkit_operator"} {
		require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{role}, PermissionWorkbenchToolsRead))
	}
	require.True(t, AllowedOrganization(ctx, a, "admin", "org-a", []string{"listingkit_admin"}, PermissionWorkbenchToolsManage))
	policy["org-a"][customizeKey] = []string{}
	require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{customizeKey}, PermissionWorkbenchToolsCustomize))
	a.SetRolePolicyReader(failedRolePolicy{})
	require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{readKey}, PermissionWorkbenchToolsRead))
}
