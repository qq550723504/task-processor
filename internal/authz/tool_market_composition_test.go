package authz

import (
	"slices"
	"testing"
)

func TestToolMarketSharedPolicyAndCustomModuleBoundary(t *testing.T) {
	a, err := NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"listingkit_viewer", "listingkit_operator", "listingkit_admin"} {
		if !a.Authorize("member", []string{role}, PermissionWorkbenchToolsRead) {
			t.Fatalf("%s cannot read enterprise tools", role)
		}
	}
	if !a.Authorize("admin", []string{"listingkit_admin"}, PermissionWorkbenchToolsManage) {
		t.Fatal("protected admin cannot manage tools")
	}
	for _, role := range []string{"listingkit_viewer", "listingkit_operator"} {
		if a.Authorize("member", []string{role}, PermissionWorkbenchToolsManage) {
			t.Fatal("non-admin can manage tools")
		}
	}
	if !ValidModuleIDs([]string{"tools", "tools-custom"}) {
		t.Fatal("approved tools modules unavailable")
	}
	granted := ModulePermissions([]string{"tools", "tools-custom"})
	for _, permission := range []string{PermissionWorkbenchToolsRead, PermissionWorkbenchToolsCustomize} {
		if !slices.Contains(granted, permission) || !slices.Contains(WorkbenchPermissions(), permission) {
			t.Fatalf("missing projection %s", permission)
		}
	}
	for _, permission := range []string{PermissionWorkbenchToolsManage, PermissionListingKitPlatformAdm, PermissionProductSourcingWrite} {
		if slices.Contains(granted, permission) {
			t.Fatalf("tool selection escalates %s", permission)
		}
	}
}
