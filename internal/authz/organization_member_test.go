package authz

import "testing"

func TestOrganizationMemberPermissionMatrix(t *testing.T) {
	for _, tc := range []struct {
		role         string
		read, manage bool
	}{
		{"listingkit_viewer", true, false},
		{"listingkit_operator", true, false},
		{"listingkit_admin", true, true},
		{"platform_admin", true, true},
		{"admin", false, false},
		{"unknown", false, false},
		{"", false, false},
	} {
		t.Run(tc.role, func(t *testing.T) {
			authorizer, err := NewListingKitAuthorizer(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for permission, want := range map[string]bool{"workbench.organization_member.read": tc.read, "workbench.organization_member.manage": tc.manage, "workbench.organization_member.unknown": false} {
				if got := authorizer.Authorize("actor", []string{tc.role}, permission); got != want {
					t.Errorf("%s=%v want %v", permission, got, want)
				}
			}
		})
	}
}

func TestConfiguredOrganizationMemberPermissionsAndRemoval(t *testing.T) {
	for _, tc := range []struct {
		name          string
		users, roles  []string
		subject, role string
	}{
		{"user only", []string{" custom-user ", "custom-user"}, nil, "custom-user", "unrelated"},
		{"role only", nil, []string{" custom-role ", "custom-role"}, "ordinary", "custom-role"},
		{"explicit admin role", nil, []string{"admin"}, "ordinary", "admin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			configured, err := NewListingKitAuthorizer(tc.users, tc.roles)
			if err != nil {
				t.Fatal(err)
			}
			revoked, err := NewListingKitAuthorizer(nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			for _, permission := range []string{"workbench.organization_member.read", "workbench.organization_member.manage"} {
				if !configured.Authorize(tc.subject, []string{tc.role}, permission) {
					t.Errorf("configured %s denied", permission)
				}
				if revoked.Authorize(tc.subject, []string{tc.role}, permission) {
					t.Errorf("removed configuration retains %s", permission)
				}
				if configured.Authorize("other-user", []string{"other-role"}, permission) {
					t.Errorf("configuration leaked %s", permission)
				}
			}
			if configured.Authorize(tc.subject, []string{tc.role}, "workbench.organization_member.unknown") {
				t.Error("unknown permission granted")
			}
		})
	}
}
