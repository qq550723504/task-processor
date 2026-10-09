package authz

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolMarketSharedPolicyAndCustomModuleBoundary(t *testing.T) {
	a, err := NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, policy := range ToolMarketPolicies() {
		if policy[0] != "listingkit_admin" {
			t.Fatalf("retired role receives static tool grant: %v", policy)
		}
	}
	ctx := context.Background()
	for _, role := range []string{"listingkit_viewer", "listingkit_operator"} {
		for _, permission := range []string{PermissionWorkbenchToolsRead, PermissionWorkbenchToolsManage, PermissionWorkbenchToolsCustomize} {
			allowed, err := a.AuthorizeScoped(ctx, "member", "org-a", []string{role}, permission)
			require.NoError(t, err)
			require.False(t, allowed, "retired role must not authorize current enterprise tools")
		}
	}
	if !ValidModuleIDs([]string{"tools", "tools-custom"}) {
		t.Fatal("approved tools modules unavailable")
	}
	granted := ModulePermissions([]string{"tools", "tools-custom"})
	for _, permission := range []string{PermissionWorkbenchToolsRead, PermissionWorkbenchToolsCustomize} {
		if !slices.Contains(granted, permission) || !slices.Contains(WorkbenchPermissions(), permission) {
			t.Fatalf("missing projection %s", permission)
		}
	}
	for _, permission := range []string{PermissionWorkbenchToolsManage, PermissionListingKitPlatformAdm, PermissionProductSourcingWrite} {
		if slices.Contains(granted, permission) {
			t.Fatalf("tool selection escalates %s", permission)
		}
	}
}

func TestToolMarketSharedCompositionUsesCurrentEnterpriseModules(t *testing.T) {
	ctx := context.Background()
	role := EnterpriseRoleKey("org-a", 1)
	policy := rolePolicyFixture{"org-a": {role: {"tools"}}}
	a, err := NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	a.SetRolePolicyReader(policy)
	check := func(org, permission string, expected bool) {
		t.Helper()
		allowed, err := a.AuthorizeScoped(ctx, "member", org, []string{role}, permission)
		require.NoError(t, err)
		require.Equal(t, expected, allowed, "%s in %s", permission, org)
	}
	check("org-a", PermissionWorkbenchToolsRead, true)
	check("org-a", PermissionWorkbenchToolsCustomize, false)
	policy["org-a"][role] = []string{"tools-custom"}
	check("org-a", PermissionWorkbenchToolsCustomize, true)
	for _, permission := range []string{PermissionWorkbenchToolsManage, PermissionListingKitPlatformAdm, PermissionProductSourcingWrite} {
		check("org-a", permission, false)
	}
	check("org-b", PermissionWorkbenchToolsRead, false)
	projected, err := a.ScopedPermissions(ctx, "member", "org-a", []string{role})
	require.NoError(t, err)
	require.Contains(t, projected, PermissionWorkbenchToolsRead)
	require.Contains(t, projected, PermissionWorkbenchToolsCustomize)
	require.NotContains(t, projected, PermissionWorkbenchToolsManage)
	policy["org-a"][role] = nil
	check("org-a", PermissionWorkbenchToolsRead, false)
	check("org-a", PermissionWorkbenchToolsCustomize, false)
	for _, permission := range []string{PermissionWorkbenchToolsRead, PermissionWorkbenchToolsManage, PermissionWorkbenchToolsCustomize} {
		allowed, err := a.AuthorizeScoped(ctx, "admin", "org-a", []string{"listingkit_admin"}, permission)
		require.NoError(t, err)
		require.True(t, allowed)
	}
}
