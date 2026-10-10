package dataservicesapp

import "task-processor/internal/authz"

// ModulePermissions supplies only this capability's menu permissions. Existing
// Collection read/manage dependencies remain explicit native module grants.
func ModulePermissions(moduleID string) []string {
	return authz.DataServicesModulePermissions(moduleID)
}
