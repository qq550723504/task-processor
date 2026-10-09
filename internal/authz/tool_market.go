package authz

const (
	PermissionWorkbenchToolsRead      = "workbench.tools.read"
	PermissionWorkbenchToolsManage    = "workbench.tools.manage"
	PermissionWorkbenchToolsCustomize = "workbench.tools.customize"
)

// ToolMarketPolicies is consumed by the shared composition Writer's existing
// Casbin policy installation. Manage is deliberately absent from module grants.
func ToolMarketPolicies() [][]string {
	return [][]string{
		{"listingkit_admin", PermissionWorkbenchToolsRead},
		{"listingkit_admin", PermissionWorkbenchToolsManage},
		{"listingkit_admin", PermissionWorkbenchToolsCustomize},
	}
}

// ToolMarketModulePermissions supplies the existing enterprise module catalog
// at shared application assembly. Native enterprise roles receive these grants
// through RoleModules; retired viewer/operator roles receive no new policy.
func ToolMarketModulePermissions(moduleID string) []string {
	switch moduleID {
	case "tools":
		return []string{PermissionWorkbenchToolsRead}
	case "tools-custom":
		return []string{PermissionWorkbenchToolsRead, PermissionWorkbenchToolsCustomize}
	default:
		return nil
	}
}
