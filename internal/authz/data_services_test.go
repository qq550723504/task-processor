package authz

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDataServicesNativeModulesPreserveSeparateGrants(t *testing.T) {
	ctx := context.Background()
	key := EnterpriseRoleKey("org-a", 1)
	policy := rolePolicyFixture{"org-a": {key: {"data-market", "data-api"}}}
	a, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	a.SetRolePolicyReader(policy)
	require.True(t, ValidModuleIDs([]string{"data-market", "data-api"}))
	require.ElementsMatch(t, []string{"workbench.data-market.use", "workbench.data-api.manage"}, ModulePermissions([]string{"data-market", "data-api"}))
	for _, p := range []string{"workbench.data-market.use", "workbench.data-api.manage"} {
		require.True(t, AllowedOrganization(ctx, a, "member", "org-a", []string{key}, p))
		require.True(t, AllowedOrganization(ctx, a, "admin", "org-a", []string{"listingkit_admin"}, p))
		require.False(t, AllowedOrganization(ctx, a, "member", "org-b", []string{key}, p))
		require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{"listingkit_viewer"}, p))
	}
	for _, p := range []string{PermissionWorkbenchCollectionManage, PermissionWorkbenchCollectionRead, PermissionListingKitPlatformAdm} {
		require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{key}, p))
	}
	policy["org-a"][key] = []string{"data-market"}
	require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{key}, "workbench.data-api.manage"))
	a.SetRolePolicyReader(failedRolePolicy{})
	require.False(t, AllowedOrganization(ctx, a, "member", "org-a", []string{key}, "workbench.data-market.use"))
}
