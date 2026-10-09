package dataservicesapp

import "task-processor/internal/dataservice"

// ModulePermissions supplies only this capability's menu permissions. Existing
// Collection read/manage dependencies remain explicit native module grants.
func ModulePermissions(moduleID string) []string {
	switch moduleID {
	case "data-market":
		return []string{dataservice.PermissionMarket}
	case "data-api":
		return []string{dataservice.PermissionManage}
	default:
		return nil
	}
}
