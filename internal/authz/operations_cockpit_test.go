package authz

import (
	"context"
	"slices"
	"testing"
)

func TestCockpitNativeModulesDoNotGrantGoalManagement(t *testing.T) {
	a := DefaultListingKitAuthorizer()
	role := EnterpriseRoleKey("org-a", 1)
	a.SetRolePolicyReader(rolePolicyFixture{"org-a": {role: {"goals", "overview-stores", "alerts", "advice"}}})
	if !ValidModuleIDs([]string{"goals", "overview-stores", "alerts", "advice"}) {
		t.Fatal("approved modules are unavailable")
	}
	for _, permission := range []string{PermissionCockpitGoalsRead, PermissionCockpitGoalsCreate, PermissionCockpitStoresRead, PermissionCockpitFactsWrite, PermissionCockpitAlertsRead, PermissionCockpitAdviceRead, PermissionWorkbenchStoreRead} {
		allowed, err := a.AuthorizeScoped(context.Background(), "member", "org-a", []string{role}, permission)
		if err != nil || !allowed || !slices.Contains(WorkbenchPermissions(), permission) {
			t.Fatalf("missing native grant %s: %v", permission, err)
		}
	}
	for _, roles := range [][]string{{role}, {"listingkit_viewer"}, {"listingkit_operator"}} {
		allowed, err := a.AuthorizeScoped(context.Background(), "member", "org-a", roles, PermissionCockpitGoalsManage)
		if err != nil || allowed {
			t.Fatalf("ordinary or retired roles gained manage: %v %v", roles, err)
		}
	}
	allowed, err := a.AuthorizeScoped(context.Background(), "admin", "org-a", []string{"listingkit_admin"}, PermissionCockpitGoalsManage)
	if err != nil || !allowed {
		t.Fatalf("current manager denied: %v", err)
	}
	allowed, err = a.AuthorizeScoped(context.Background(), "member", "org-b", []string{role}, PermissionCockpitGoalsRead)
	if err != nil || allowed {
		t.Fatal("cross-org native role gained access")
	}
}
