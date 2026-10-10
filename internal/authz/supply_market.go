package authz

// Feature-local enterprise grants never include platform publication authority.
var supplyMarketPermissions = []string{"workbench.supply-market.read", "workbench.supply-market.select", "workbench.supply-market.apply", "workbench.supply-market.design"}

func supplyMarketModulePermissions() []string {
	return append(append([]string{}, supplyMarketPermissions...), PermissionWorkbenchCollectionRead, PermissionWorkbenchCollectionManage)
}
