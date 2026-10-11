package authz

const (
	PermissionWorkbenchDataMarket = "workbench.data-market.use"
	PermissionWorkbenchDataAPI    = "workbench.data-api.manage"
)

// Data Services grants remain separate from the Collection module.
func DataServicesModulePermissions(moduleID string) []string {
	switch moduleID {
	case "data-market":
		return []string{PermissionWorkbenchDataMarket}
	case "data-api":
		return []string{PermissionWorkbenchDataAPI}
	default:
		return nil
	}
}

func DataServicesPolicies() [][]string {
	return [][]string{{"listingkit_admin", PermissionWorkbenchDataMarket}, {"listingkit_admin", PermissionWorkbenchDataAPI}}
}
