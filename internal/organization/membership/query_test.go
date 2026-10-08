package membership

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/authz"
)

type queryDirectory struct {
	directoryStub
	query PageRequest
}

func (d *queryDirectory) List(ctx context.Context, org string, request PageRequest) (Page, error) {
	d.query = request
	return d.directoryStub.List(ctx, org, request)
}

func TestFilteredDirectoryRejectsInvalidQueryBeforeRead(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	for _, filter := range []ListFilter{{Role: "platform_admin"}, {Role: "unknown"}, {State: "pending"}, {State: "ACTIVE"}, {Search: "a\x00b"}, {Search: string([]byte{255})}, {Search: strings.Repeat("a", 201)}, {Search: strings.Repeat("目", 67)}} {
		d := &directoryStub{}
		result, err := testService(d, a, "project").List(scopedContext(testReadRole), PageRequest{Limit: 20, Filter: filter})
		if err != ErrInvalidRequest || d.calls != 0 || len(result.Items) != 0 {
			t.Fatalf("query=%+v err=%v calls=%d", filter, err, d.calls)
		}
	}
}

func TestDirectorySearchLiteralAndUnicodeBounds(t *testing.T) {
	for _, tc := range []struct {
		q, name string
		match   bool
	}{
		{"%_", "literal %_ name", true}, {"%_", "wildcard other name", false}, {"目标", "目标成员", true},
		{"ÄBC", "äbc", true}, {"é", "e", false}, {"\u2003target\u2003", "TARGET@example.invalid", true},
		{"ος", "ΟΣ", true}, {"S", "ſ", true}, {"STRASSE", "Straße", true},
	} {
		p, err := (PageRequest{Limit: 20, Filter: ListFilter{Search: tc.q}}).Normalize()
		for _, member := range []Member{{LoginName: tc.name}, {DisplayName: tc.name}} {
			if err != nil || p.Filter.Matches(member) != tc.match {
				t.Fatalf("case=%+v member=%+v err=%v", tc, member, err)
			}
		}
	}
	if _, err := (PageRequest{Limit: 20, Filter: ListFilter{Search: strings.Repeat("目", 66) + "ab"}}).Normalize(); err != nil {
		t.Fatal("200-byte query rejected", err)
	}
}

func TestFilteredDirectoryEnforcesQueryAndOriginalCapabilities(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	member := Member{ID: "grant", UserID: "user", OrganizationID: "effective-b", ProjectID: "project", DisplayName: "目标成员", LoginName: "TARGET@example.invalid", Roles: []string{testReadRole, testOperateRole}, State: "inactive"}
	filter := ListFilter{Search: " target ", Role: testOperateRole, State: "inactive"}
	d := &queryDirectory{directoryStub: directoryStub{page: Page{Items: []Member{member}, Total: 1}}}
	result, err := testService(d, a, "project").List(scopedContext(testReadRole), PageRequest{Limit: 20, Filter: filter})
	if err != nil || d.query.Filter.Search != "target" || result.CanManage || len(result.AssignableRoles) != 0 || result.Items[0].CanRemove {
		t.Fatalf("result=%+v query=%+v err=%v", result, d.query, err)
	}
	for _, change := range []func(*Member){func(m *Member) { m.LoginName = "unmatched" }, func(m *Member) { m.Roles = []string{testReadRole} }, func(m *Member) { m.State = "active" }, func(m *Member) { m.OrganizationID = "other" }, func(m *Member) { m.ProjectID = "other" }} {
		wrong := member
		change(&wrong)
		d.page = Page{Items: []Member{wrong}, Total: 1}
		if _, err := testService(d, a, "project").List(scopedContext(testReadRole), PageRequest{Limit: 20, Filter: filter}); err != ErrInvalidResponse {
			t.Fatalf("published mismatched member: %+v err=%v", wrong, err)
		}
	}
}

func TestProtectedCurrentRolesRemainFilterableWithoutBecomingAssignable(t *testing.T) {
	for _, role := range []string{"listingkit_admin", testOperateRole} {
		t.Run(role, func(t *testing.T) {
			a, err := authz.NewListingKitAuthorizer(nil, []string{role})
			if err != nil {
				t.Fatal(err)
			}
			d := &queryDirectory{directoryStub: directoryStub{page: Page{Items: []Member{{ID: "grant", UserID: "protected", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{role}, State: "active"}}, Total: 1}}}
			s := testService(d, a, "project", role)
			result, err := s.List(scopedContext("listingkit_admin"), PageRequest{Limit: 20, Filter: ListFilter{Role: role}})
			if err != nil || d.calls != 1 || len(result.Items) != 1 {
				t.Fatalf("role=%s result=%+v reads=%d err=%v", role, result, d.calls, err)
			}
			for _, assignable := range result.AssignableRoles {
				if assignable == role {
					t.Fatal("protected role became assignable")
				}
			}
			if result.Items[0].CanChangeRole || result.Items[0].CanRemove || s.ValidInvitationRole(context.Background(), "effective-b", role) {
				t.Fatal("protected member or role became editable")
			}
		})
	}
}
