package membership

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type directoryStub struct {
	page         Page
	err          error
	calls        int
	organization string
	onRead       func()
}

func (d *directoryStub) List(_ context.Context, organization string, _ PageRequest) (Page, error) {
	if d.onRead != nil {
		d.onRead()
	}
	d.calls++
	d.organization = organization
	return d.page, d.err
}

func scopedContext(role string) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		UserID: "actor", HomeOrganizationID: "home-a", EffectiveOrganizationID: "effective-b",
		Roles: []string{role}, TokenExpiresAt: time.Now().Add(time.Hour),
		OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "effective-b", ProjectID: "project", Roles: []string{role}}},
	})
}

func (d *directoryStub) Read(_ context.Context, organization, id string) (Member, error) {
	if d.onRead != nil {
		d.onRead()
	}
	d.calls++
	d.organization = organization
	if d.err != nil {
		return Member{}, d.err
	}
	for _, member := range d.page.Items {
		if member.ID == id {
			return member, nil
		}
	}
	return Member{}, ErrNotFound
}

func TestTokenExpiryDuringDirectoryReadDiscardsResult(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	for _, detail := range []bool{false, true} {
		expires := time.Now().Add(10 * time.Millisecond)
		ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "actor", EffectiveOrganizationID: "effective-b", Roles: []string{"listingkit_admin"}, TokenExpiresAt: expires, OrganizationGrants: []authidentity.OrganizationGrant{{ProjectID: "project", OrganizationID: "effective-b", Roles: []string{"listingkit_admin"}}}})
		d := &directoryStub{page: Page{Total: 1, Items: []Member{{ID: "member", UserID: "user", OrganizationID: "effective-b", ProjectID: "project"}}}, onRead: func() { time.Sleep(time.Until(expires) + time.Millisecond) }}
		s := NewService(d, a, "project")
		var err error
		if detail {
			_, err = s.Read(ctx, "member")
		} else {
			_, err = s.List(ctx, PageRequest{Limit: 20})
		}
		if err != ErrAuthentication {
			t.Fatalf("expired response returned: detail=%v err=%v", detail, err)
		}
	}
}

func TestReadMemberRejectsRemovedAndCrossOrganization(t *testing.T) {
	authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
	directory := &directoryStub{page: Page{Items: []Member{{ID: "target", UserID: "member", OrganizationID: "effective-b", ProjectID: "project"}}, Total: 1}}
	service := NewService(directory, authorizer, "project")
	result, err := service.Read(scopedContext("listingkit_operator"), "target")
	if err != nil || len(result.Items) != 1 || result.Items[0].ID != "target" || result.CanManage {
		t.Fatalf("detail: %+v %v", result, err)
	}
	directory.page.Items[0].OrganizationID = "foreign"
	if result, err := service.Read(scopedContext("listingkit_operator"), "target"); err != ErrInvalidResponse || len(result.Items) != 0 {
		t.Fatalf("foreign member accepted: %+v %v", result, err)
	}
	directory.page.Items = nil
	if _, err := service.Read(scopedContext("listingkit_operator"), "target"); err != ErrNotFound {
		t.Fatalf("removed member: %v", err)
	}
}

func TestListBindsEffectiveOrganizationAndProjectsPermissions(t *testing.T) {
	for _, role := range []string{"listingkit_viewer", "listingkit_operator", "listingkit_admin"} {
		t.Run(role, func(t *testing.T) {
			directory := &directoryStub{page: Page{Items: []Member{{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}, State: "active"}}, Total: 1}}
			authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
			service := NewService(directory, authorizer, "project")
			result, err := service.List(scopedContext(role), PageRequest{Limit: 20})
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if directory.organization != "effective-b" || result.OrganizationID != "effective-b" || result.UserID != "actor" || len(result.Items) != 1 {
				t.Fatalf("wrong scope/result: %+v, directory=%s", result, directory.organization)
			}
			if result.CanManage != (role == "listingkit_admin") {
				t.Fatalf("role %s manage=%v", role, result.CanManage)
			}
		})
	}
}

func TestListRejectsUnverifiedOrExpiredIdentityBeforeDirectory(t *testing.T) {
	for _, scenario := range []string{"missing", "expired", "ungranted", "wrong-project", "unknown-role"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := scopedContext("listingkit_viewer")
			identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
			switch scenario {
			case "missing":
				ctx = context.Background()
			case "expired":
				identity.TokenExpiresAt = time.Now().Add(-time.Second)
			case "ungranted":
				identity.OrganizationGrants = nil
			case "wrong-project":
				identity.OrganizationGrants[0].ProjectID = "another-project"
			case "unknown-role":
				identity.Roles = []string{"not-a-member"}
				identity.OrganizationGrants[0].Roles = identity.Roles
			}
			if scenario != "missing" {
				ctx = authidentity.WithAuthenticatedIdentity(ctx, identity)
			}
			directory := &directoryStub{}
			authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
			_, err := NewService(directory, authorizer, "project").List(ctx, PageRequest{Limit: 20})
			if err == nil || directory.calls != 0 {
				t.Fatalf("unauthorized access: err=%v calls=%d", err, directory.calls)
			}
		})
	}
}

func TestListRejectsCrossScopeProviderResponse(t *testing.T) {
	for _, field := range []string{"organization", "project"} {
		t.Run(field, func(t *testing.T) {
			member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}, State: "active"}
			if field == "organization" {
				member.OrganizationID = "home-a"
			} else {
				member.ProjectID = "foreign-project"
			}
			directory := &directoryStub{page: Page{Items: []Member{member}, Total: 1}}
			authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
			result, err := NewService(directory, authorizer, "project").List(scopedContext("listingkit_admin"), PageRequest{Limit: 20})
			if !errors.Is(err, ErrInvalidResponse) || len(result.Items) != 0 {
				t.Fatalf("cross-scope response leaked: %+v %v", result, err)
			}
		})
	}
}

func TestListDependencyFailureIsNotEmpty(t *testing.T) {
	directory := &directoryStub{err: errors.New("private provider detail")}
	authorizer, _ := authz.NewListingKitAuthorizer(nil, nil)
	_, err := NewService(directory, authorizer, "project").List(scopedContext("listingkit_viewer"), PageRequest{Limit: 20})
	if !errors.Is(err, ErrUnavailable) || err.Error() != ErrUnavailable.Error() {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestConfiguredPlatformCapabilityStillRequiresExactOrganizationGrant(t *testing.T) {
	for _, byUser := range []bool{false, true} {
		t.Run(map[bool]string{true: "configured user", false: "configured role"}[byUser], func(t *testing.T) {
			var users, roles []string
			if byUser {
				users = []string{"actor"}
			} else {
				roles = []string{"custom-platform-operator"}
			}
			authorizer, _ := authz.NewListingKitAuthorizer(users, roles)
			ctx := scopedContext("custom-platform-operator")
			directory := &directoryStub{page: Page{Items: []Member{}, Total: 0}}
			result, err := NewService(directory, authorizer, "project").List(ctx, PageRequest{Limit: 20})
			if err != nil || !result.CanManage {
				t.Fatalf("configured capability not projected: %+v %v", result, err)
			}
			identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
			identity.EffectiveOrganizationID = "foreign-org"
			_, err = NewService(directory, authorizer, "project").List(authidentity.WithAuthenticatedIdentity(ctx, identity), PageRequest{Limit: 20})
			if err == nil || directory.calls != 1 {
				t.Fatalf("configured administrator bypassed org grant: %v calls=%d", err, directory.calls)
			}
		})
	}
}

type captureAuthorizer struct{ permissions []string }

func (a *captureAuthorizer) Authorize(_ string, _ []string, permission string) bool {
	a.permissions = append(a.permissions, permission)
	return permission == "workbench.organization_member.read"
}

func TestListUsesOnlyExplicitMembershipCapabilities(t *testing.T) {
	authorizer := &captureAuthorizer{}
	directory := &directoryStub{page: Page{Items: []Member{}, Total: 0}}
	result, err := NewService(directory, authorizer, "project").List(scopedContext("listingkit_admin"), PageRequest{Limit: 20})
	if err != nil || result.CanManage {
		t.Fatalf("role substituted for authority: %+v %v", result, err)
	}
	read, manage := false, false
	for _, permission := range authorizer.permissions {
		switch permission {
		case "workbench.organization_member.read":
			read = true
		case "workbench.organization_member.manage":
			manage = true
		default:
			t.Fatalf("unrelated permission requested: %s", permission)
		}
	}
	if !read || !manage {
		t.Fatalf("missing permission projection: %+v", authorizer.permissions)
	}
}
