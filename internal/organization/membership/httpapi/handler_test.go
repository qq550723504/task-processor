package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/organization/membership"
)

type directory struct {
	calls int
	err   error
}

func TestDirectoryQueryParameterBounds(t *testing.T) {
	for _, query := range []string{"q=a&q=b", "role=", "role=platform_admin", "role=listingkit_viewer&role=listingkit_admin", "state=pending", "state=", "q=%00", "q=%FF", "q=%zz", "q=" + url.QueryEscape(strings.Repeat("目", 67)), "q=" + strings.Repeat("a", 201), "q=x&projectId=other", "q=x;state=active", "q=" + strings.Repeat("a", 2049)} {
		u := &url.URL{RawQuery: query}
		if _, err := parsePage(u); err != membership.ErrInvalidRequest {
			t.Fatalf("accepted query %q: %v", query, err)
		}
	}
	for _, query := range []string{"q=", "q=+++", "q=" + url.QueryEscape(strings.Repeat("目", 66)+"ab")} {
		if _, err := parsePage(&url.URL{RawQuery: query}); err != nil {
			t.Fatalf("rejected valid query %q: %v", query, err)
		}
	}
	p, err := parsePage(&url.URL{RawQuery: "limit=1&offset=20&q=%E7%9B%AE%E6%A0%87&role=listingkit_operator&state=inactive"})
	if err != nil || p.Limit != 1 || p.Offset != 20 || p.Filter.Search != "目标" || p.Filter.Role != "listingkit_operator" || p.Filter.State != "inactive" {
		t.Fatalf("lost query: %+v %v", p, err)
	}
}

func TestCommandRoutesHaveExactManagePermission(t *testing.T) {
	reg := kernelmodule.NewRegistry()
	h := NewCommandHandler(nil, func(*http.Request) (CommandService, error) { return &commandStub{}, nil })
	if err := NewModule(h).Register(reg); err != nil {
		t.Fatal(err)
	}
	routes := reg.Routes()
	if len(routes) != 8 {
		t.Fatalf("routes=%d", len(routes))
	}
	for _, route := range routes[3:] {
		if route.Permission != authz.PermissionWorkbenchOrganizationMemberManage || route.AuthPolicy != httproute.AuthPolicyCurrentIdentity || route.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || route.OrganizationTargetResolver == nil {
			t.Fatalf("unprotected command: %+v", route)
		}
	}
}

func (d *directory) List(_ context.Context, org string, p membership.PageRequest) (membership.Page, error) {
	d.calls++
	if org != "org-b" || p.Limit != 20 || p.Offset != 0 {
		panic("unexpected provider scope/page")
	}
	return membership.Page{Items: []membership.Member{}, Total: 0}, d.err
}
func (d *directory) Read(_ context.Context, org, id string) (membership.Member, error) {
	d.calls++
	return membership.Member{}, membership.ErrNotFound
}

func TestReadHTTPBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, path, org, body, role string
		status, calls               int
	}{
		{"list", "/api/v1/account/members", "org-b", "", "listingkit_viewer", 200, 1},
		{"filtered list", "/api/v1/account/members?q=Member&role=listingkit_viewer&state=active", "org-b", "", "listingkit_viewer", 200, 1},
		{"detail absent", "/api/v1/account/members/absent", "org-b", "", "listingkit_operator", 404, 1},
		{"no selector", "/api/v1/account/members", "", "", "listingkit_viewer", 400, 0},
		{"different selector", "/api/v1/account/members", "org-a", "", "listingkit_viewer", 403, 0},
		{"unknown query", "/api/v1/account/members?organizationId=org-a", "org-b", "", "listingkit_viewer", 400, 0},
		{"duplicate limit", "/api/v1/account/members?limit=20&limit=1", "org-b", "", "listingkit_viewer", 400, 0},
		{"invalid escape", "/api/v1/account/members?limit=%zz", "org-b", "", "listingkit_viewer", 400, 0},
		{"excess limit", "/api/v1/account/members?limit=101", "org-b", "", "listingkit_viewer", 400, 0},
		{"body", "/api/v1/account/members", "org-b", "{}", "listingkit_viewer", 400, 0},
		{"detail query", "/api/v1/account/members/absent?limit=20", "org-b", "", "listingkit_viewer", 400, 0},
		{"bare admin denied", "/api/v1/account/members", "org-b", "", "admin", 403, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &directory{}
			a, _ := authz.NewListingKitAuthorizer(nil, nil)
			h := NewHandler(membership.NewService(d, a, "project"))
			router := gin.New()
			router.GET("/api/v1/account/members", h.List)
			router.GET("/api/v1/account/members/:member_id", h.Read)
			r := httptest.NewRequest("GET", tc.path, strings.NewReader(tc.body))
			r.Header.Set("X-Requested-Organization-ID", tc.org)
			r = r.WithContext(authidentity.WithAuthenticatedIdentity(r.Context(), authidentity.AuthenticatedIdentity{UserID: "actor", EffectiveOrganizationID: "org-b", HomeOrganizationID: "org-a", Roles: []string{tc.role}, TokenExpiresAt: time.Now().Add(time.Hour), OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org-b", ProjectID: "project", Roles: []string{tc.role}}}}))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != tc.status || d.calls != tc.calls {
				t.Fatalf("status=%d calls=%d body=%s", w.Code, d.calls, w.Body.String())
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("cacheable membership response")
			}
		})
	}
}

func TestMemberRoutesRequireCurrentIdentityAndLiveGrants(t *testing.T) {
	reg := kernelmodule.NewRegistry()
	if err := NewModule(NewHandler(nil)).Register(reg); err != nil {
		t.Fatal(err)
	}
	routes := reg.Routes()
	if len(routes) != 3 {
		t.Fatalf("routes=%d", len(routes))
	}
	for _, r := range routes {
		if r.Method != "GET" || r.AuthPolicy != httproute.AuthPolicyCurrentIdentity || r.OrganizationAccessPolicy != httproute.OrganizationAccessPolicyLiveWrite || r.Permission != authz.PermissionWorkbenchOrganizationMemberRead || r.OrganizationTargetResolver == nil || !r.RejectUnreadRequestBody || r.RequestTimeout <= 0 {
			t.Fatalf("unprotected route: %+v", r)
		}
		request := httptest.NewRequest("GET", r.Path, nil)
		request.Header.Add("X-Requested-Organization-ID", "org-b")
		request.Header.Add("X-Requested-Organization-ID", "org-a")
		if _, err := r.OrganizationTargetResolver(request); err == nil {
			t.Fatal("duplicate organization accepted")
		}
	}
}
