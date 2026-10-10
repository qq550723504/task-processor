package authz

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSupplyMarketModulesDoNotGrantPlatformAuthority(t *testing.T) {
	for _, id := range []string{"supply-official", "supply-selected", "supply-catalogs"} {
		require.True(t, ValidModuleIDs([]string{id}))
		p := ModulePermissions([]string{id})
		require.Contains(t, p, "workbench.supply-market.read")
		require.Contains(t, p, "workbench.supply-market.select")
		require.Contains(t, p, PermissionWorkbenchCollectionManage)
		require.NotContains(t, p, PermissionListingKitPlatformAdm)
	}
	a := DefaultListingKitAuthorizer()
	require.True(t, a.Authorize("actor", []string{"listingkit_operator"}, "workbench.supply-market.design"))
	require.True(t, a.Authorize("actor", []string{"listingkit_viewer"}, "workbench.supply-market.read"))
	require.False(t, a.Authorize("actor", []string{"listingkit_viewer"}, "workbench.supply-market.design"))
	require.False(t, a.Authorize("actor", []string{"listingkit_admin"}, PermissionListingKitPlatformAdm))
}

func TestSupplyMarketKeepsConfiguredPlatformAdminUserPolicy(t *testing.T) {
	a, err := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	require.NoError(t, err)
	for _, permission := range supplyMarketPermissions {
		for _, subject := range []struct {
			user  string
			roles []string
		}{{"configured-user", nil}, {"configured-user", []string{"listingkit_viewer"}}, {"actor", []string{"configured-role"}}} {
			allowed, err := a.AuthorizeScoped(context.Background(), subject.user, "org-a", subject.roles, permission)
			require.NoError(t, err)
			require.True(t, allowed, permission)
		}
		allowed, err := a.AuthorizeScoped(context.Background(), "ordinary-user", "org-a", nil, permission)
		require.NoError(t, err)
		require.False(t, allowed, permission)
		require.Equal(t, permission == "workbench.supply-market.read", a.Authorize("ordinary-user", []string{"listingkit_viewer"}, permission), permission)
	}
}
