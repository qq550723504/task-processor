package membership

import (
	"slices"
	"task-processor/internal/authz"
	"testing"
)

func TestInactiveMemberHasNoEffectivePermissions(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	d := &directoryStub{page: Page{Total: 2, Items: []Member{{ID: "active", UserID: "u1", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{testOperateRole}, State: "active"}, {ID: "inactive", UserID: "u2", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_admin"}, State: "inactive"}}}}
	result, err := testService(d, a, "project").List(scopedContext("listingkit_admin"), PageRequest{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(result.Items[0].Permissions, authz.PermissionWorkbenchStoreCreate) || len(result.Items[1].Permissions) != 0 {
		t.Fatalf("permissions %+v", result.Items)
	}
}

func TestConfiguredPlatformRolesAreNeverAssignableOrEditable(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, []string{testOperateRole})
	d := &directoryStub{page: Page{Total: 2, Items: []Member{
		{ID: "one", UserID: "u1", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{testOperateRole}},
		{ID: "two", UserID: "u2", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{testReadRole}},
	}}}
	s := testService(d, a, "project", testOperateRole)
	result, err := s.List(scopedContext("listingkit_admin"), PageRequest{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AssignableRoles) != 2 || !slices.Contains(result.AssignableRoles, testReadRole) || !slices.Contains(result.AssignableRoles, "listingkit_admin") {
		t.Fatalf("options=%v", result.AssignableRoles)
	}
	if result.Items[0].CanChangeRole || result.Items[0].CanRemove || !result.Items[1].CanChangeRole || !result.Items[1].CanRemove {
		t.Fatalf("capabilities=%+v", result.Items)
	}
	readonly, err := s.List(scopedContext(testReadRole), PageRequest{Limit: 20})
	if err != nil || len(readonly.AssignableRoles) != 0 || readonly.Items[1].CanChangeRole || readonly.Items[1].CanRemove {
		t.Fatalf("readonly=%+v err=%v", readonly, err)
	}
}

func TestObservedVersionIsStableAndTracksAssignmentFacts(t *testing.T) {
	m := Member{ID: "grant", UserID: "user", OrganizationID: "org", ProjectID: "project", Roles: []string{"listingkit_admin", testReadRole}, State: "active", ChangedAt: "2026-09-12T00:00:00Z"}
	original := observedVersion(m)
	m.Roles = []string{testReadRole, "listingkit_admin"}
	if observedVersion(m) != original {
		t.Fatal("role order changed version")
	}
	m.DisplayName = "Changed display"
	if observedVersion(m) != original {
		t.Fatal("display-only change changed authorization version")
	}
	m.ChangedAt = "2026-09-12T00:01:00Z"
	if observedVersion(m) == original {
		t.Fatal("assignment change did not change version")
	}
}
