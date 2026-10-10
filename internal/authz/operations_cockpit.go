package authz

const (
	PermissionCockpitGoalsRead   = "workbench.cockpit.goals.read"
	PermissionCockpitGoalsCreate = "workbench.cockpit.goals.create"
	PermissionCockpitGoalsManage = "workbench.cockpit.goals.manage"
	PermissionCockpitStoresRead  = "workbench.cockpit.stores.read"
	PermissionCockpitFactsWrite  = "workbench.cockpit.facts.write"
	PermissionCockpitAlertsRead  = "workbench.cockpit.alerts.read"
	PermissionCockpitAdviceRead  = "workbench.cockpit.advice.read"
)

var cockpitPermissions = []string{PermissionCockpitGoalsRead, PermissionCockpitGoalsCreate, PermissionCockpitGoalsManage, PermissionCockpitStoresRead, PermissionCockpitFactsWrite, PermissionCockpitAlertsRead, PermissionCockpitAdviceRead}

func CockpitModulePermissions(id string) []string {
	switch id {
	case "goals":
		return []string{PermissionCockpitGoalsRead, PermissionCockpitGoalsCreate, PermissionWorkbenchStoreRead}
	case "overview-stores":
		return []string{PermissionCockpitStoresRead, PermissionCockpitFactsWrite, PermissionWorkbenchStoreRead}
	case "alerts":
		return []string{PermissionCockpitAlertsRead, PermissionWorkbenchStoreRead}
	case "advice":
		return []string{PermissionCockpitAdviceRead, PermissionWorkbenchStoreRead}
	default:
		return nil
	}
}

func CockpitPolicies() [][]string {
	result := [][]string{}
	for _, p := range cockpitPermissions {
		result = append(result, []string{"listingkit_admin", p})
	}
	return result
}
