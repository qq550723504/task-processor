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
		{"listingkit_viewer", PermissionWorkbenchToolsRead},
		{"listingkit_operator", PermissionWorkbenchToolsRead},
		{"listingkit_operator", PermissionWorkbenchToolsCustomize},
		{"listingkit_admin", PermissionWorkbenchToolsRead},
		{"listingkit_admin", PermissionWorkbenchToolsManage},
		{"listingkit_admin", PermissionWorkbenchToolsCustomize},
	}
}
