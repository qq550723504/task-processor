package authz

import (
	"context"
	"github.com/stretchr/testify/require"
	"slices"
	"testing"
)

func TestStoreObservationRolesAndScopedModules(t *testing.T) {
	a, err := NewListingKitAuthorizer(nil, []string{"configured-admin"})
	require.NoError(t, err)
	for _, role := range []string{"listingkit_viewer", "listingkit_operator", "listingkit_admin", "platform_admin", "configured-admin"} {
		for _, kind := range []string{"products", "orders"} {
			read, sync := "workbench.store."+kind+".read", "workbench.store."+kind+".sync"
			require.True(t, a.Authorize("actor", []string{role}, read), role+read)
			require.Equal(t, role != "listingkit_viewer", a.Authorize("actor", []string{role}, sync), role+sync)
			require.True(t, slices.Contains(WorkbenchPermissions(), read))
			require.True(t, slices.Contains(WorkbenchPermissions(), sync))
		}
	}
	key := EnterpriseRoleKey("org-a", 1)
	policy := rolePolicyFixture{"org-a": {key: {"store-orders"}}}
	a.SetRolePolicyReader(policy)
	for _, permission := range []string{PermissionWorkbenchStoreRead, "workbench.store.orders.read", "workbench.store.orders.sync"} {
		allowed, err := a.AuthorizeScoped(context.Background(), "member", "org-a", []string{key}, permission)
		require.NoError(t, err)
		require.True(t, allowed, permission)
	}
	for _, permission := range []string{"workbench.store.products.read", PermissionWorkbenchSupplyRead, PermissionWorkbenchListingSubmit, PermissionWorkbenchStoreUpdate, PermissionWorkbenchStoreLifecycle, PermissionWorkbenchStoreDelete} {
		allowed, err := a.AuthorizeScoped(context.Background(), "member", "org-a", []string{key}, permission)
		require.NoError(t, err)
		require.False(t, allowed, permission)
	}
	policy["org-a"][key] = nil
	allowed, err := a.AuthorizeScoped(context.Background(), "member", "org-a", []string{key}, "workbench.store.orders.read")
	require.NoError(t, err)
	require.False(t, allowed)
	require.True(t, ValidModuleIDs([]string{"store-products", "store-orders"}))
}

func TestConfiguredStoreObservationAdminUserKeepsCurrentPolicy(t *testing.T) {
	a, err := NewListingKitAuthorizer([]string{"configured-user"}, nil)
	require.NoError(t, err)
	for _, permission := range []string{"workbench.store.products.read", "workbench.store.products.sync", "workbench.store.orders.read", "workbench.store.orders.sync"} {
		allowed, err := a.AuthorizeScoped(context.Background(), "configured-user", "org-a", nil, permission)
		require.NoError(t, err)
		require.True(t, allowed, permission)
		allowed, err = a.AuthorizeScoped(context.Background(), "ordinary-user", "org-a", nil, permission)
		require.NoError(t, err)
		require.False(t, allowed, permission)
	}
}
