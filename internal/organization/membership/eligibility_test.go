package membership

import (
	"task-processor/internal/authz"
	"testing"
)

func TestConfiguredPlatformRolesAreNeverAssignableOrEditable(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, []string{"listingkit_operator"})
	d := &directoryStub{page: Page{Total: 2, Items: []Member{
		{ID: "one", UserID: "u1", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_operator"}},
		{ID: "two", UserID: "u2", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}},
	}}}
	s := NewService(d, a, "project", "listingkit_operator")
	result, err := s.List(scopedContext("listingkit_admin"), PageRequest{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AssignableRoles) != 2 || result.AssignableRoles[0] != "listingkit_viewer" || result.AssignableRoles[1] != "listingkit_admin" {
		t.Fatalf("options=%v", result.AssignableRoles)
	}
	if result.Items[0].CanChangeRole || result.Items[0].CanRemove || !result.Items[1].CanChangeRole || !result.Items[1].CanRemove {
		t.Fatalf("capabilities=%+v", result.Items)
	}
	readonly, err := s.List(scopedContext("listingkit_viewer"), PageRequest{Limit: 20})
	if err != nil || len(readonly.AssignableRoles) != 0 || readonly.Items[1].CanChangeRole || readonly.Items[1].CanRemove {
		t.Fatalf("readonly=%+v err=%v", readonly, err)
	}
}

func TestObservedVersionIsStableAndTracksAssignmentFacts(t *testing.T) {
	m := Member{ID: "grant", UserID: "user", OrganizationID: "org", ProjectID: "project", Roles: []string{"listingkit_admin", "listingkit_viewer"}, State: "active", ChangedAt: "2026-09-12T00:00:00Z"}
	original := observedVersion(m)
	m.Roles = []string{"listingkit_viewer", "listingkit_admin"}
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
