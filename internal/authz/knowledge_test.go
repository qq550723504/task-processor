package authz

import "testing"

func TestKnowledgeRolesUseCurrentWorkbenchPolicy(t *testing.T) {
	a, err := NewListingKitAuthorizer([]string{"configured-user"}, []string{"configured-role"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		role         string
		read, manage bool
	}{{"listingkit_viewer", false, false}, {"listingkit_operator", true, false}, {"listingkit_admin", true, true}, {"platform_admin", true, true}, {"admin", false, false}, {"configured-role", true, true}} {
		for permission, want := range map[string]bool{"workbench.knowledge.read": tt.read, "workbench.knowledge.manage": tt.manage} {
			if got := a.Authorize("actor", []string{tt.role}, permission); got != want {
				t.Errorf("role %s permission %s got %v want %v", tt.role, permission, got, want)
			}
		}
	}
	for _, permission := range []string{"workbench.knowledge.read", "workbench.knowledge.manage"} {
		if !a.Authorize("configured-user", nil, permission) {
			t.Errorf("configured user denied %s", permission)
		}
	}
}
