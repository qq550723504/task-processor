package authz

import (
	"slices"
	"testing"
)

func TestAgentCustomizationMenuUsesEnterprisePermissionsOnly(t *testing.T) {
	if !ValidModuleIDs([]string{"agent-custom"}) {
		t.Fatal("delivered module unavailable")
	}
	permissions := ModulePermissions([]string{"agent-custom"})
	if !slices.Contains(permissions, PermissionWorkbenchAgentRead) || !slices.Contains(permissions, PermissionWorkbenchAgentUse) || slices.Contains(permissions, PermissionListingKitPlatformAdm) {
		t.Fatalf("incorrect enterprise permissions %v", permissions)
	}
}
