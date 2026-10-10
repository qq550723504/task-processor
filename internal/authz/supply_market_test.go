package authz

import (
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
